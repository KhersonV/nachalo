package repository

import (
 "context"
 "database/sql"
 "fmt"
)

// MarketPurchaseRequirementError describes an authoritative purchase restriction.
type MarketPurchaseRequirementError struct { Building string; Level int }
func (e *MarketPurchaseRequirementError) Error() string {
 if e.Building=="forge" {
  if e.Level>1 {return fmt.Sprintf("Для покупки требуется Кузница %d уровня.",e.Level)}
  return "Для покупки этого чертежа необходимо построить Кузницу."
 }
 return "Требование для покупки не выполнено."
}

// The market catalog is populated from the shop's blueprint metadata. Usage
// requirements on equipment are intentionally not purchase restrictions.
func marketPurchaseEligible(ctx context.Context,tx *sql.Tx,userID int,kind,stackKey string) error {
 if kind!="stack" {return nil}
 var required int
 err:=tx.QueryRowContext(ctx,`SELECT required_forge_level FROM market_stack_catalog WHERE inventory_key=$1`,stackKey).Scan(&required)
 if err==sql.ErrNoRows{return ErrMarketInvalid};if err!=nil{return err}
 if required<=0{return nil}
 var actual int
 err=tx.QueryRowContext(ctx,`SELECT forge_level FROM player_base_buildings WHERE user_id=$1 FOR SHARE`,userID).Scan(&actual)
 if err==sql.ErrNoRows{actual=0}else if err!=nil{return err}
 if actual<required{return &MarketPurchaseRequirementError{Building:"forge",Level:required}}
 return nil
}

func MarketPurchaseState(ctx context.Context,userID int)(int,error){
 var level int
 err:=DB.QueryRowContext(ctx,`SELECT COALESCE((SELECT forge_level FROM player_base_buildings WHERE user_id=$1),0)`,userID).Scan(&level)
 return level,err
}
