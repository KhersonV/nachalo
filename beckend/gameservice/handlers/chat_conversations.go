package handlers

import (
    "encoding/json"
    "io"
    "net/http"
    "strconv"
    "strings"

    "gameservice/middleware"
    "gameservice/repository"
    "github.com/gorilla/mux"
)

func conversationUser(w http.ResponseWriter,r *http.Request) (int,bool) {
    id,ok:=middleware.GetUserIDFromContext(r.Context())
    if !ok || id<=0 { http.Error(w,"unauthorized",http.StatusUnauthorized); return 0,false }
    return id,true
}

func conversationID(w http.ResponseWriter,r *http.Request) (int64,bool) {
    id,err:=strconv.ParseInt(mux.Vars(r)["id"],10,64)
    if err!=nil || id<=0 { http.Error(w,"invalid conversation",http.StatusBadRequest); return 0,false }
    return id,true
}

func ChatConversationsHandler(w http.ResponseWriter,r *http.Request) {
    userID,ok:=conversationUser(w,r); if !ok { return }
    store:=repository.ChatStore{DB:repository.DB}
    if r.Method==http.MethodGet {
        list,err:=store.ListConversations(r.Context(),userID)
        if err!=nil { chatError(w,err); return }
        writeChatJSON(w,http.StatusOK,map[string]any{"conversations":list})
        return
    }
    r.Body=http.MaxBytesReader(w,r.Body,4096)
    decoder:=json.NewDecoder(r.Body); decoder.DisallowUnknownFields()
    var request struct { RecipientIDs []int `json:"recipientIds"` }
    if err:=decoder.Decode(&request); err!=nil { http.Error(w,"invalid participants",http.StatusBadRequest); return }
    var extra any
    if err:=decoder.Decode(&extra); err!=io.EOF { http.Error(w,"invalid participants",http.StatusBadRequest); return }
    if len(request.RecipientIDs)<1 || len(request.RecipientIDs)>49 { http.Error(w,"select 1–49 recipients",http.StatusBadRequest); return }
    seen:=map[int]bool{userID:true}
    for _,id:=range request.RecipientIDs { if id<=0 || seen[id] { http.Error(w,"invalid or duplicate participant",http.StatusBadRequest); return }; seen[id]=true }
    conversation,err:=store.CreateConversation(r.Context(),userID,request.RecipientIDs)
    if err!=nil { chatError(w,err); return }
    writeChatJSON(w,http.StatusCreated,conversation)
}

func ChatConversationMessagesHandler(w http.ResponseWriter,r *http.Request) {
    userID,ok:=conversationUser(w,r); if !ok { return }
    id,ok:=conversationID(w,r); if !ok { return }
    before:=int64(0); limit:=40
    if raw:=r.URL.Query().Get("beforeId"); raw!="" { var err error; before,err=strconv.ParseInt(raw,10,64); if err!=nil || before<=0 { http.Error(w,"invalid cursor",http.StatusBadRequest); return } }
    if raw:=r.URL.Query().Get("limit"); raw!="" { var err error; limit,err=strconv.Atoi(raw); if err!=nil || limit<1 || limit>100 { http.Error(w,"invalid limit",http.StatusBadRequest); return } }
    store:=repository.ChatStore{DB:repository.DB}
    messages,err:=store.History(r.Context(),repository.ChatHistoryQuery{Channel:"PRIVATE",ViewerUserID:userID,ConversationID:id,BeforeID:before,Limit:limit})
    if err!=nil { chatError(w,err); return }
    response:=chatHistoryResponse{Messages:messages}
    if len(messages)==limit { next:=messages[len(messages)-1].ID; response.NextBeforeID=&next }
    writeChatJSON(w,http.StatusOK,response)
}

func ChatConversationReadHandler(w http.ResponseWriter,r *http.Request) {
    userID,ok:=conversationUser(w,r); if !ok { return }
    id,ok:=conversationID(w,r); if !ok { return }
    store:=repository.ChatStore{DB:repository.DB}
    if err:=store.MarkConversationRead(r.Context(),userID,id); err!=nil { chatError(w,err); return }
    w.WriteHeader(http.StatusNoContent)
}

func ChatConversationLeaveHandler(w http.ResponseWriter,r *http.Request) {
    userID,ok:=conversationUser(w,r); if !ok { return }
    id,ok:=conversationID(w,r); if !ok { return }
    store:=repository.ChatStore{DB:repository.DB}
    if err:=store.LeaveConversation(r.Context(),userID,id); err!=nil { chatError(w,err); return }
    w.WriteHeader(http.StatusNoContent)
}

func ChatPlayerSearchHandler(w http.ResponseWriter,r *http.Request) {
    userID,ok:=conversationUser(w,r); if !ok { return }
    term:=strings.TrimSpace(r.URL.Query().Get("search"))
    if len([]rune(term))>100 { http.Error(w,"search too long",http.StatusBadRequest); return }
    store:=repository.ChatStore{DB:repository.DB}
    players,err:=store.SearchChatPlayers(r.Context(),userID,term)
    if err!=nil { chatError(w,err); return }
    writeChatJSON(w,http.StatusOK,map[string]any{"players":players})
}
