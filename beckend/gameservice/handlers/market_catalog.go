package handlers

import (
 "context"
 "gameservice/repository"
)

func RegisterMarketShopCatalog(ctx context.Context) error {
 definitions:=make([]repository.MarketStackDefinition,0,len(blueprintCatalog)+len(scrollCatalog))
 for _,item:=range blueprintCatalog {requiredForgeLevel:=0;if item.RequiresForge{requiredForgeLevel=1};definitions=append(definitions,repository.MarketStackDefinition{InventoryKey:item.InventoryKey,ID:item.ID,Code:item.Type,Name:item.Name,Category:"recipe",Subcategory:"blueprint",Rarity:"gray",ImageURL:item.Image,Tradable:!item.Untradable,Stackable:true,RequiredForgeLevel:requiredForgeLevel})}
 for _,item:=range scrollCatalog {definitions=append(definitions,repository.MarketStackDefinition{InventoryKey:item.InventoryKey,ID:item.ID,Code:item.Type,Name:item.Name,Category:"consumable",Subcategory:"scroll",Rarity:"gray",ImageURL:item.Image,Tradable:!item.Untradable,Stackable:true})}
 return repository.SyncMarketStackCatalog(ctx,definitions)
}
