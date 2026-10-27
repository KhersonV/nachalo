package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"gameservice/repository"
)

// The route uses the lobby's existing authentication. Only public definitions
// are returned: no account identifiers, stock, private messages, RNG or ENV.
type knowledgeShopItem struct {
	*shopItem
	RequiresLibrary bool `json:"requiresLibrary"`
}

func GetKnowledgeHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	data, err := repository.KnowledgeCatalog(ctx)
	if err != nil {
		log.Printf("knowledge config: %v", err)
		http.Error(w, "knowledge unavailable", http.StatusServiceUnavailable)
		return
	}
	data.Facts = append(data.Facts,
		repository.KnowledgeFact{ID: "pve-guardian", Title: "Страж против монстра", Text: "Если Страж пережил состоявшийся ближний ответ монстра, применяется его отдельный ответный урон по живой цели. Это правило PvE не следует подменять формулой геройской контратаки.", Formula: "max(1, DEF Стража − floor(эффективная DEF монстра / 2))"},
		repository.KnowledgeFact{ID: "pve-berserker", Title: "Берсерк против монстра", Text: fmt.Sprintf("Ответный урон монстра по Берсерку до входящих защит умножается на %d%% с усечением; положительный результат не ниже единицы. Дополнительный удар требует пережить этот обмен.", berserkerMonsterCounterPercent)},
		repository.KnowledgeFact{ID: "pve-mystic", Title: "Мистик против монстра", Text: fmt.Sprintf("В основном PvE-ударе используется %d%% исходной DEF монстра с целочисленным усечением. Физическую DEF не применяют второй раз к дополнительной магии.", mysticMonsterDefensePercent)},
		repository.KnowledgeFact{ID: "ranger-chain", Title: "Серия Рейнджера", Text: fmt.Sprintf("Максимум собственных стаков пробивания: %d. Следующее неблокированное дальнее попадание по живой цели разрешает отталкивание. В v34 при PvP-столкновении базовый урон считается с DEF цели, уменьшенной на 4, затем применяются правила дополнительного повреждения без Ненависти и рекурсивных основных срабатываний.", armorBreakMaxStacks)},
	)
	shop := []knowledgeShopItem{}
	for _, code := range append([]string{"food", "water"}, repository.JewelryShopTypes()...) {
		item, e := getShopItem(code)
		if e != nil {
			data.Unavailable = append(data.Unavailable, "Магазин: "+code)
			continue
		}
		shop = append(shop, knowledgeShopItem{item, item.Category == "scroll"})
	}
	for code := range blueprintCatalog {
		if item, ok := getBlueprintItem(code); ok {
			shop = append(shop, knowledgeShopItem{item, item.Category == "scroll"})
		}
	}
	for code := range scrollCatalog {
		if item, ok := getScrollItem(code); ok {
			shop = append(shop, knowledgeShopItem{item, item.Category == "scroll"})
		}
	}
	sort.Slice(shop, func(i, j int) bool { return shop[i].Type < shop[j].Type })
	payload := struct {
		repository.KnowledgeData
		Shop []knowledgeShopItem `json:"shop"`
	}{data, shop}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("knowledge encode: %v", err)
		http.Error(w, "knowledge unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(body)
}
