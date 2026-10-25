package handlers

import (
 "encoding/json"
 "errors"
 "io"
 "net/http"
 "strconv"

 "gameservice/middleware"
 "gameservice/repository"
)
func artifactDismantleError(w http.ResponseWriter,err error) {
 status:=http.StatusInternalServerError
 if errors.Is(err,repository.ErrDismantleInvalid) { status=http.StatusBadRequest }
 if errors.Is(err,repository.ErrDismantleMatch) || errors.Is(err,repository.ErrDismantleReplay) { status=http.StatusConflict }
 message:="artifact_dismantle_unavailable"
 if status!=http.StatusInternalServerError {message=err.Error()}
 w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(map[string]string{"error":message})
}
func ArtifactDismantleHandler(w http.ResponseWriter,r *http.Request) {
 user,ok:=middleware.GetUserIDFromContext(r.Context());if !ok || user<=0 {http.Error(w,"unauthorized",401);return}
 w.Header().Set("Cache-Control","no-store")
 if r.Method==http.MethodGet {
  artifact,e:=strconv.Atoi(r.URL.Query().Get("artifactId"));if e!=nil {artifactDismantleError(w,repository.ErrDismantleInvalid);return}
  quantity:=1
  if q:=r.URL.Query().Get("quantity");q!="" { quantity,e=strconv.Atoi(q);if e!=nil {artifactDismantleError(w,repository.ErrDismantleInvalid);return} }
  out,e:=repository.PreviewArtifactDismantle(r.Context(),user,artifact,quantity);if e!=nil {artifactDismantleError(w,e);return}
  w.Header().Set("Content-Type","application/json");_ = json.NewEncoder(w).Encode(out);return
 }
 var req struct {ArtifactID int `json:"artifactId"`;Quantity int `json:"quantity"`;CommandID string `json:"commandId"`}
 decoder:=json.NewDecoder(http.MaxBytesReader(w,r.Body,4096));decoder.DisallowUnknownFields()
 if e:=decoder.Decode(&req);e!=nil {artifactDismantleError(w,repository.ErrDismantleInvalid);return}
 var trailing any
 if e:=decoder.Decode(&trailing);e!=io.EOF {artifactDismantleError(w,repository.ErrDismantleInvalid);return}
 out,e:=repository.DismantleArtifact(r.Context(),user,req.ArtifactID,req.Quantity,req.CommandID);if e!=nil {artifactDismantleError(w,e);return}
 w.Header().Set("Content-Type","application/json");_ = json.NewEncoder(w).Encode(out)
}
