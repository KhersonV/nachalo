package handlers

import (
 "encoding/json"
 "database/sql"
 "errors"
 "log"
 "net/http"
 "regexp"
 "strconv"
 "strings"

 "gameservice/middleware"
 "gameservice/repository"
 "github.com/gorilla/mux"
)
var marketUUID=regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
type marketRequest struct {Kind string `json:"kind"`;ItemInstanceID string `json:"itemInstanceId"`;ResourceID int `json:"resourceId"`;TemplateID int64 `json:"templateId"`;StackKey string `json:"stackKey"`;Quantity int `json:"quantity"`;StartPrice int64 `json:"startPrice"`;BuyoutPrice int64 `json:"buyoutPrice"`;Duration int `json:"duration"`;Amount int64 `json:"amount"`;PricePerUnit int64 `json:"pricePerUnit"`;ItemKey string `json:"itemKey"`;Name string `json:"name"`;Filters json.RawMessage `json:"filters"`}
func marketJSON(w http.ResponseWriter,status int,value any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(value)}
func marketError(w http.ResponseWriter,err error){var requirement *repository.MarketPurchaseRequirementError;if errors.As(err,&requirement){marketJSON(w,http.StatusForbidden,map[string]string{"error":"purchase_requirement","message":requirement.Error()});return};code:=http.StatusInternalServerError;message:="market_server_error";switch{case errors.Is(err,repository.ErrMarketRate):code=http.StatusTooManyRequests;message="Слишком много действий. Попробуйте позже.";case errors.Is(err,sql.ErrNoRows):code=http.StatusNotFound;message="Не найдено.";case errors.Is(err,repository.ErrMarketInvalid):code=http.StatusBadRequest;message="invalid_request";case errors.Is(err,repository.ErrMarketUnavailable):code=http.StatusConflict;message="listing_unavailable";case errors.Is(err,repository.ErrMarketForbidden):code=http.StatusForbidden;message="forbidden";case errors.Is(err,repository.ErrMarketFunds):code=http.StatusConflict;message="not_enough_gold";case errors.Is(err,repository.ErrMarketInventory):code=http.StatusConflict;message="item_unavailable";case errors.Is(err,repository.ErrMarketLimit):code=http.StatusConflict;message="market_limit";case errors.Is(err,repository.ErrMarketIdempotency):code=http.StatusConflict;message="idempotency_conflict";default:log.Printf("market request failed: %v",err)};marketJSON(w,code,map[string]string{"error":message})}
func marketInt(raw string,minimum,maximum int)(int,error){v,e:=strconv.Atoi(raw);if e!=nil||v<minimum||v>maximum{return 0,repository.ErrMarketInvalid};return v,nil}
func marketPage(r *http.Request)(repository.MarketFilter,error){q:=r.URL.Query();f:=repository.MarketFilter{Search:strings.TrimSpace(q.Get("search")),Category:q.Get("category"),Subcategory:q.Get("subcategory"),Rarity:q.Get("rarity"),Class:q.Get("class"),Status:q.Get("status"),Sort:q.Get("sort"),ItemKind:q.Get("itemKind"),ItemKey:q.Get("itemKey"),Party:strings.TrimSpace(q.Get("party")),Page:1,Limit:20};var e error
 for _,field:=range []struct{key string;dest *int;max int}{{"page",&f.Page,100000},{"limit",&f.Limit,50},{"minLevel",&f.MinLevel,10000},{"maxLevel",&f.MaxLevel,10000}}{if q.Get(field.key)!=""{*field.dest,e=marketInt(q.Get(field.key),0,field.max);if e!=nil{return f,e}}};if f.Page<1||f.Limit<1{return f,repository.ErrMarketInvalid}
 if q.Get("targetId")!=""{f.TargetID,e=strconv.ParseInt(q.Get("targetId"),10,64);if e!=nil||f.TargetID<1{return f,repository.ErrMarketInvalid}};if q.Get("minPrice")!=""{f.MinPrice,e=strconv.ParseInt(q.Get("minPrice"),10,64);if e!=nil{return f,repository.ErrMarketInvalid}};if q.Get("maxPrice")!=""{f.MaxPrice,e=strconv.ParseInt(q.Get("maxPrice"),10,64);if e!=nil{return f,repository.ErrMarketInvalid}};return f,f.Validate()}
func marketBody(w http.ResponseWriter,r *http.Request)(marketRequest,error){var body marketRequest;r.Body=http.MaxBytesReader(w,r.Body,8192);decoder:=json.NewDecoder(r.Body);decoder.DisallowUnknownFields();if e:=decoder.Decode(&body);e!=nil{return body,repository.ErrMarketInvalid};return body,nil}
func marketActor(w http.ResponseWriter,r *http.Request)(int,bool){id,ok:=middleware.GetUserIDFromContext(r.Context());if !ok||id<=0{marketJSON(w,http.StatusUnauthorized,map[string]string{"error":"unauthorized"});return 0,false};return id,true}
func marketKey(r *http.Request)string{key:=strings.TrimSpace(r.Header.Get("Idempotency-Key"));if !marketUUID.MatchString(key){return ""};return key}
func marketID(r *http.Request)(int64,error){id,e:=strconv.ParseInt(mux.Vars(r)["id"],10,64);if e!=nil||id<=0{return 0,repository.ErrMarketInvalid};return id,nil}
func MarketReadHandler(w http.ResponseWriter,r *http.Request){userID,ok:=marketActor(w,r);if !ok{return};if e:=repository.MarketRateCheck(r.Context(),userID,"api");e!=nil{marketError(w,e);return};f,e:=marketPage(r);if e!=nil{marketError(w,e);return};kind:=mux.Vars(r)["kind"];scope:=r.URL.Query().Get("scope");if scope==""{scope="all"};if scope!="all"&&scope!="mine"&&scope!="bids"&&scope!="related"{marketError(w,repository.ErrMarketInvalid);return};if kind=="orders"&&scope=="bids"{marketError(w,repository.ErrMarketInvalid);return}
 switch kind{case "notifications-count":items,e:=repository.MarketNotificationUnreadCount(r.Context(),userID);if e!=nil{marketError(w,e)}else{marketJSON(w,200,map[string]any{"unread":items})};return
 case "notifications":items,count,e:=repository.MarketNotifications(r.Context(),userID,f.Page,f.Limit);if e!=nil{marketError(w,e)}else{marketJSON(w,200,map[string]any{"items":items,"unread":count})};return
 case "favorites":items,e:=repository.MarketFavorites(r.Context(),userID);if e!=nil{marketError(w,e)}else{marketJSON(w,200,map[string]any{"items":items})};return
 case "saved-searches":items,e:=repository.MarketSavedSearches(r.Context(),userID);if e!=nil{marketError(w,e)}else{marketJSON(w,200,map[string]any{"items":items})};return
 case "sales":items,e:=repository.MarketCompletedSales(r.Context(),f.Page,f.Limit);if e!=nil{marketError(w,e)}else{marketJSON(w,200,map[string]any{"items":items})};return
 case "price":summary,e:=repository.MarketPriceSummary(r.Context(),r.URL.Query());if e!=nil{marketError(w,e)}else{marketJSON(w,200,summary)};return
 case "categories":items,e:=repository.MarketCategories(r.Context());if e!=nil{marketError(w,e)}else{marketJSON(w,200,map[string]any{"items":items})};return
 case "preview":gross,parseErr:=strconv.ParseInt(r.URL.Query().Get("gross"),10,64);if parseErr!=nil{marketError(w,repository.ErrMarketInvalid);return};preview,previewErr:=repository.MarketPricePreview(gross);if previewErr!=nil{marketError(w,previewErr)}else{marketJSON(w,200,preview)};return
 case "eligibility":forgeLevel,eligibilityErr:=repository.MarketPurchaseState(r.Context(),userID);if eligibilityErr!=nil{marketError(w,eligibilityErr)}else{marketJSON(w,200,map[string]any{"forgeLevel":forgeLevel})};return
 case "config":marketJSON(w,200,map[string]any{"auctionDurations":[]int{6,12,24},"exchangeDuration":24,"auctionLimit":repository.MarketAuctionLimit,"orderLimit":repository.MarketOrderLimit,"feePercent":repository.MarketFeePercent,"minimumBidIncrement":repository.MarketMinBidIncrement,"maxPrice":repository.MarketMaxPrice});return
 case "auctions":items,e:=repository.MarketAuctions(r.Context(),userID,scope,f);if e!=nil{marketError(w,e)}else{marketJSON(w,200,map[string]any{"items":items,"page":f.Page,"limit":f.Limit})};return
 case "orders":items,e:=repository.MarketOrders(r.Context(),userID,scope,f);if e!=nil{marketError(w,e)}else{marketJSON(w,200,map[string]any{"items":items,"page":f.Page,"limit":f.Limit})};return
 case "catalog":items,e:=repository.MarketCatalog(r.Context(),f);if e!=nil{marketError(w,e)}else{marketJSON(w,200,map[string]any{"items":items,"page":f.Page,"limit":f.Limit})};return
 case "inventory":items,e:=repository.MarketInventory(r.Context(),userID);if e!=nil{marketError(w,e)}else{marketJSON(w,200,map[string]any{"items":items})};return
 case "deliveries":items,e:=repository.MarketDeliveries(r.Context(),userID,f.Page,f.Limit);if e!=nil{marketError(w,e)}else{marketJSON(w,200,map[string]any{"items":items,"page":f.Page,"limit":f.Limit})};return
 case "wallet-claims":items,e:=repository.MarketWalletClaims(r.Context(),userID,f.Page,f.Limit);if e!=nil{marketError(w,e)}else{marketJSON(w,200,map[string]any{"items":items,"page":f.Page,"limit":f.Limit})};return
 case "history":items,e:=repository.MarketTransactions(r.Context(),userID,f.Page,f.Limit);if e!=nil{marketError(w,e)}else{marketJSON(w,200,map[string]any{"items":items,"page":f.Page,"limit":f.Limit})};return}
 marketJSON(w,404,map[string]string{"error":"not_found"})}
func MarketActionHandler(w http.ResponseWriter,r *http.Request){userID,ok:=marketActor(w,r);if !ok{return};if e:=repository.MarketRateCheck(r.Context(),userID,"api");e!=nil{marketError(w,e);return};key:=marketKey(r);if key==""{marketError(w,repository.ErrMarketInvalid);return};kind:=mux.Vars(r)["kind"];action:=mux.Vars(r)["action"];var body marketRequest;var e error;if action!="cancel"&&action!="claim"&&action!="remove"&&action!="delete"&&action!="read"{body,e=marketBody(w,r);if e!=nil{marketError(w,e);return}};var id int64;if mux.Vars(r)["id"]!=""{id,e=marketID(r);if e!=nil{marketError(w,e);return}}
 if body.ItemInstanceID!=""&&!marketUUID.MatchString(body.ItemInstanceID){marketError(w,repository.ErrMarketInvalid);return}
 rateCategory:="";if action=="create"{rateCategory="create"}else if action=="cancel"{rateCategory="cancel"}else if action=="bid"{rateCategory="bid"};if rateCategory!=""{if e=repository.MarketRateCheck(r.Context(),userID,rateCategory);e!=nil{marketError(w,e);return}}
 var result int64;switch{case kind=="notifications"&&action=="read":e=repository.MarketReadNotification(r.Context(),userID,id)
 case kind=="favorites"&&action=="add":e=repository.MarketFavoriteChange(r.Context(),userID,body.Kind,body.ItemKey,true)
 case kind=="favorites"&&action=="remove":parts:=strings.SplitN(r.URL.Query().Get("item"),":",2);if len(parts)!=2{e=repository.ErrMarketInvalid}else{e=repository.MarketFavoriteChange(r.Context(),userID,parts[0],parts[1],false)}
 case kind=="saved-searches"&&action=="create":result,e=repository.MarketSaveSearch(r.Context(),userID,body.Name,body.Filters)
 case kind=="saved-searches"&&action=="rename":e=repository.MarketRenameSearch(r.Context(),userID,id,body.Name)
 case kind=="saved-searches"&&action=="delete":e=repository.MarketDeleteSearch(r.Context(),userID,id)
case kind=="auctions"&&action=="create":result,e=repository.MarketCreateAuction(r.Context(),userID,key,body.Kind,body.ItemInstanceID,body.StackKey,body.ResourceID,body.Quantity,body.StartPrice,body.BuyoutPrice,body.Duration)
 case kind=="auctions"&&action=="bid":result,e=repository.MarketBid(r.Context(),userID,key,id,body.Amount)
 case kind=="auctions"&&action=="buyout":result,e=repository.MarketBuyout(r.Context(),userID,key,id)
 case kind=="auctions"&&action=="cancel":result,e=repository.MarketCancelAuction(r.Context(),userID,key,id)
 case kind=="orders"&&action=="create":if body.Kind=="resource"||body.Kind==""{result,e=repository.MarketCreateOrder(r.Context(),userID,key,body.ResourceID,body.Quantity,body.PricePerUnit)}else{result,e=repository.MarketCreateOrderItem(r.Context(),userID,key,body.Kind,body.StackKey,body.TemplateID,body.Quantity,body.PricePerUnit)}
 case kind=="orders"&&action=="fulfill":result,e=repository.MarketFulfillOrder(r.Context(),userID,key,id,body.Quantity,body.ItemInstanceID)
 case kind=="orders"&&action=="cancel":result,e=repository.MarketCancelOrder(r.Context(),userID,key,id)
 case kind=="wallet-claims"&&action=="claim":result,e=repository.MarketClaimGold(r.Context(),userID,key,id)
 case kind=="deliveries"&&action=="claim":result,e=repository.MarketClaimDelivery(r.Context(),userID,key,id)
 default:marketJSON(w,404,map[string]string{"error":"not_found"});return}
 if e!=nil{if errors.Is(e,repository.ErrMarketForbidden)&&(action=="bid"||action=="buyout"||action=="fulfill"){repository.MarketAudit(r.Context(),userID,"forbidden_trade_attempt",kind,id,map[string]any{"action":action})};marketError(w,e);return};marketJSON(w,200,map[string]any{"id":result,"status":"ok"})}
