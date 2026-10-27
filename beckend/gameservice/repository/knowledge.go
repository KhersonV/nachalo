package repository

// Presentation-only catalogue. Never joins player inventories, matches, orders,
// messages or profiles; never executes schema setup or gameplay mutations.
import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strings"

	"gameservice/game"
	"gameservice/internal/game/balance"
)

type KnowledgeFact struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Text    string `json:"text"`
	Formula string `json:"formula,omitempty"`
}
type KnowledgeHero struct {
	Code  string         `json:"code"`
	Name  string         `json:"name"`
	Image string         `json:"image"`
	Stats EquipmentStats `json:"stats"`
}
type KnowledgeMonster struct {
	EquipmentChance float64 `json:"equipmentChance"`
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	Image           string  `json:"image"`
	HP              int     `json:"hp"`
	Attack          int     `json:"attack"`
	Defense         int     `json:"defense"`
	Speed           int     `json:"speed"`
	Maneuverability int     `json:"maneuverability"`
	Vision          int     `json:"vision"`
}
type KnowledgeResource struct {
	ID          int    `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Image       string `json:"image"`
	Tradable    bool   `json:"tradable"`
}
type KnowledgeArtifact struct {
	ID          int             `json:"id"`
	Code        string          `json:"code"`
	Description string          `json:"description"`
	Image       string          `json:"image"`
	Bonus       json.RawMessage `json:"bonus"`
}
type KnowledgeBuilding struct {
	Code  string         `json:"code"`
	Name  string         `json:"name"`
	Costs map[string]int `json:"costs"`
}

// Category probabilities, not a probability for every catalogue item or HEX cell.
type KnowledgeLootRules struct {
	UncommonEquipmentChance             float64 `json:"uncommonEquipmentChance"`
	BarrelArtifactChance                float64 `json:"barrelArtifactChance"`
	BarrelDamageChance                  float64 `json:"barrelDamageChance"`
	BarrelResourceChance                float64 `json:"barrelResourceChance"`
	BarrelEmptyChance                   float64 `json:"barrelEmptyChance"`
	QuestArtifactGuaranteedInLastBarrel bool    `json:"questArtifactGuaranteedInLastBarrel"`
}

func knowledgeLootRules() KnowledgeLootRules {
	return KnowledgeLootRules{
		UncommonEquipmentChance:             float64(EquipmentDropChancePercent()) / 100,
		BarrelArtifactChance:                float64(game.BarrelArtifactChancePercent()) / 100,
		BarrelDamageChance:                  float64(game.BarrelDamageChancePercent) / 100,
		BarrelResourceChance:                float64(game.BarrelResourceChancePercent) / 100,
		BarrelEmptyChance:                   float64(100-game.BarrelDamageChancePercent-game.BarrelResourceChancePercent-game.BarrelArtifactChancePercent()) / 100,
		QuestArtifactGuaranteedInLastBarrel: true,
	}
}

type KnowledgeData struct {
	Loot              KnowledgeLootRules           `json:"loot"`
	Ruleset           balance.CombatRuleset        `json:"ruleset"`
	Balance           *balance.V34ResearchConfig   `json:"balance,omitempty"`
	Heroes            []KnowledgeHero              `json:"heroes"`
	Facts             []KnowledgeFact              `json:"facts"`
	Equipment         []*EquipmentItem             `json:"equipment"`
	Recipes           []CraftRecipe                `json:"recipes"`
	Resources         []KnowledgeResource          `json:"resources"`
	Monsters          []KnowledgeMonster           `json:"monsters"`
	Drops             []ProfessionLootMonster      `json:"drops"`
	Gathering         []GatheringCategory          `json:"gathering"`
	Professions       []ProfessionDefinition       `json:"professions"`
	Artifacts         []KnowledgeArtifact          `json:"artifacts"`
	Dismantle         map[string][]CraftIngredient `json:"dismantle"`
	Buildings         []KnowledgeBuilding          `json:"buildings"`
	Unavailable       []string                     `json:"unavailable"`
	Sources           []string                     `json:"sources"`
	MapResources      []string                     `json:"mapResources"`
	ItemSources       map[string]string            `json:"itemSources"`
	ReflexProgression map[string]map[string]int    `json:"reflexProgression,omitempty"`
}

// These formulas are explanations, not an alternative combat implementation.
// Configurable values and numeric examples come from the live server helpers.
func knowledgeFacts(ruleset balance.CombatRuleset, cfg *balance.V34ResearchConfig) []KnowledgeFact {
	loot := knowledgeLootRules()
	facts := []KnowledgeFact{
		{ID: "equipment-loot", Title: "Зелёная добыча с монстров", Text: fmt.Sprintf("Одно подтверждённое убийство монстра: один независимый бросок категории Uncommon с шансом %g%%. При успехе выдаётся один предмет из разрешённого зелёного пула основных комплектов всех классов. Это не бросок на каждую вещь. Кольца, амулеты и синие предметы в этот пул не входят. Дополнительного требования по рангу, профессии или виду монстра у этого броска нет. Для убийств турелью требуется допустимый владелец. Профессиональные материалы разыгрываются отдельно по своим ограничениям.", 100*loot.UncommonEquipmentChance)},
		{ID: "barrel-loot", Title: "Обычная добыча из бочек", Text: fmt.Sprintf("Одно открытие обычной бочки: один выбор взаимоисключающей категории — урон %g%%, обычный ресурс %g%%, обычный артефакт %g%%, пусто %g%%. После успеха выбирается один артефакт из допустимого каталога, без выбранного квестового вида матча. При пустом пуле повторного броска нет. Монстры, узлы сбора и отдельные HEX-клетки такого артефактного броска не дают.", 100*loot.BarrelDamageChance, 100*loot.BarrelResourceChance, 100*loot.BarrelArtifactChance, 100*loot.BarrelEmptyChance)},
		{ID: "quest-loot", Title: "Квестовая гарантия", Text: "При создании HEX-матча выбирается квестовый вид артефакта, но предмет не выдаётся на старте. Последняя неоткрытая бочка гарантирует единственный квестовый экземпляр, если он ещё не появился. Это существующее исключение из обычного шанса бочки. Его вынос завершает квест; сам квестовый экземпляр не переносится на постоянный склад и не даёт материалов разбора."},
		{ID: "jewelry-materials", Title: "Материалы украшений", Text: "Осколки, зачарованная пыль, arcane_fragment и refined_essence получаются из разбора допустимых обычных артефактов после выноса и вне матча. Выход разбора зависит от вида артефакта и показан в его карточке. Заготовки покупаются отдельно. Rare-кольца и амулеты требуют кузнеца ранга Мастер и построенную Гильдию; цены и состав рецептов не изменены."},
		{ID: "turntimer", Title: "Время хода", Text: fmt.Sprintf("Серверный лимит хода для новых матчей: %g секунд. Следите за таймером текущей экспедиции и не рассчитывайте на автоматический обмен оставшихся ОД.", HexTurnDuration().Seconds())},
		{ID: "hp", Title: "HP и MaxHP", Text: "HP — оставшееся здоровье. Лечение ограничено MaxHP. При смерти герой исключается из очереди действий; судьбу добычи определяют правила выхода и временного рюкзака."},
		{ID: "attack", Title: "ATK", Text: "ATK участвует в основном уроне. Фиксированная магия и аура в v34 масштабируются от текущего ATK один раз. Бонусы надетых вещей суммируются; вещи на складе усиления не дают."},
		{ID: "budgets", Title: "Действия и движение", Text: fmt.Sprintf("Базовый запас: %g ОД. Обычная атака стоит 1 ОД, шаг — 0.5 ОП. Базовый MaxMP равен Mobility в экранных ОП; общий запас движения ограничен %g ОП. Обмен: 1 ОД → 2 ОП. AP=0 при оставшихся MP сам по себе не завершает ход.", float64(HexMaxAP)/2, float64(HexMPCap)/2), Formula: "Начало личного хода → восстановление MaxMP; обмен: 1 ОД → 2 ОП"},
		{ID: "reactions", Title: "Реакции", Text: "Ближний удар по живому герою может вызвать контратаку, если у него остались реакции. В HEX герой тратит реакцию, а не отдельные ОД на обычную контратаку. Ответы монстров PvE описаны отдельно. Дальний удар не вызывает обычную ближнюю контратаку. Погибший от магии Страж уже не контратакует; погибший от контратаки герой не продолжает действия."},
		{ID: "vision", Title: "Обзор и дальность", Text: "Обзор открывает клетки и цели, дальность ограничивает атаку. Стены, занятые клетки и тип клетки влияют на маршрут и допустимость действий. Видимая цель не обязательно доступна для атаки."},
		{ID: "pockets", Title: "Карманы", Text: fmt.Sprintf("База — %d. Только надетые зелёные нагрудник и штаны дают по +1, синие — по +2. Полный синий набор даёт %d карманов при системном пределе %d. Это характеристики вещей, не бонус полного комплекта. Перед матчем проверьте выбранные расходники; предметы сверх текущей вместимости использовать нельзя.", HexBasePockets, HexBasePockets+4, HexMaxPockets)},
		{ID: "portal", Title: "Возвращение и временная добыча", Text: fmt.Sprintf("Возвращайтесь через портал: с квестовым артефактом либо платным выходом за %d золота, если выполнены условия режима. Только успешная эвакуация переносит разрешённую временную добычу и профессиональный XP. При смерти временный рюкзак теряется; постоянный склад — отдельное хранилище. Не путайте обычный трофей с единственным квестовым артефактом матча.", hexPaidExtractionCost)},
		{ID: "profession", Title: "Профессии и опыт", Text: fmt.Sprintf("Всего %d ранга: Ученик, Ремесленник, Мастер. Накопительные пороги: %d и %d XP. Мирный сбор: до %d успехов за матч, по %d XP. Боевая профессиональная добыча имеет независимый лимит %d. Гильдия должна существовать до начала матча; добывающая профессия зависит от класса.", ProfessionMaxLevel, ProfessionCraftspersonXP, ProfessionMasterXP, ProfessionGatherLimit, ProfessionGatherXP, ProfessionMonsterDropLimit)},
		{ID: "market", Title: "Торговля и комиссия", Text: fmt.Sprintf("Комиссия удерживается с продавца после продажи: %d%% с округлением вверх до целого золота. Одновременно разрешено до %d аукционов и %d заявок. Аукцион — 6/12/24 часа; рыночная заявка — 24 часа. Повышение ставки — не менее %d золота. Количество — до %d; цена единицы — до %d; сумма сделки — до %d золота. Сумма продажи должна превышать комиссию. Не все предметы допустимы к продаже: важны привязка, состояние, блокировки и прочность.", MarketFeePercent, MarketAuctionLimit, MarketOrderLimit, MarketMinBidIncrement, MarketMaxQuantity, MarketMaxPrice, MarketMaxTotal), Formula: fmt.Sprintf("Комиссия = ceil(сумма продажи × %d / 100)", MarketFeePercent)},
	}
	if cfg == nil {
		return append(facts, KnowledgeFact{ID: "damage", Title: "Защита: legacy", Text: "Новые матчи сервера сейчас используют legacy. Сохранённые матчи могут иметь другую конфигурацию. Классовые коэффициенты и предметные улучшения задаются старым профилем.", Formula: "ATK > 0: max(1, floor(ATK × 20 / (20 + max(0, DEF))))"}, KnowledgeFact{ID: "reflex", Title: "Reflex в legacy", Text: "Базовый шанс зависит от Agility; в PvP дополнительно действует прежнее классовое противостояние. Предметные значения в каталоге показаны для активного ruleset.", Formula: "min(25, floor(30 × Reflex / (Reflex + 10)))%"})
	}
	p := balance.DefaultCombatProfile()
	chanceRows := []string{}
	for _, reflex := range []int{5, 15, 20, 30} {
		chance := balance.CalculateReflexProcChance(reflex)
		block := balance.ResearchRound(float64(chance) * cfg.Factors["block"])
		if block > 55 {
			block = 55
		}
		chanceRows = append(chanceRows, fmt.Sprintf("Reflex %d: классовый шанс %d%%, блок %d%%", reflex, chance, block))
	}
	penetration := "Физический блок обнуляет основной удар Мистика, но засчитывает попадание серии и пропускает Overload/магический Reflex. Полное dodge не продвигает серию и не допускает магию этой атаки. Щиты и правила смерти продолжают действовать; физические множители основного урона не применяются к магии повторно."
	if !cfg.MysticMagicThroughBlock {
		penetration = "Для новых матчей этой конфигурации прохождение магии через блок не включено. Используйте опубликованные правила активного профиля, не перенося на него механику другой версии."
	}
	return append(facts,
		KnowledgeFact{ID: "damage", Title: "Основной урон и DEF", Text: "Округление — к ближайшему целому; половины к чётному (roundEven). При ATK ≤ 0 урон нулевой. Защита уменьшает основной удар; она не применяется второй раз к дополнительной магии.", Formula: fmt.Sprintf("max(1, roundEven(ATK − %.4f × max(0, DEF)))", float64(cfg.FlatDefenseBasisPoints)/10000)},
		KnowledgeFact{ID: "reflex", Title: "Reflex / Agility", Text: strings.Join(chanceRows, " · ") + ". Это пороги вероятности, не гарантия срабатывания. Показанный Reflex полного синего набора с украшениями — цель комплектации, не предел характеристики.", Formula: "Шанс = min(25, floor(30 × Reflex / (Reflex + 10)))%; блок = min(55, roundEven(шанс × " + fmt.Sprint(cfg.Factors["block"]) + "))%"},
		KnowledgeFact{ID: "hatred", Title: "Круг Ненависти", Text: fmt.Sprintf("Страж → Берсерк → Рейнджер → Мистик → Страж. +%d%% относится только к основной атаке: не к контратаке, ауре, перегрузке или другим дополнительным повреждениям.", cfg.HatredPercent), Formula: fmt.Sprintf("floor(основной урон × %d / 100)", 100+cfg.HatredPercent)},
		KnowledgeFact{ID: "guardian", Title: "Страж: блок, контратака и аура", Text: fmt.Sprintf("Радиус базовой ауры %d HEX. Урон начала хода %d; выход — %d до масштаба ATK. Выход из объединения вражеских аур учитывается один раз. Добровольное движение владельца не ранит врага уходящей аурой; принудительное перемещение учитывается. Первый шаг из чужой ауры за личный ход требует ещё 1 ОП.", p.AuraRadius, p.AuraTick, balance.ResearchRound(float64(p.AuraExit)*cfg.Factors["aura"])), Formula: fmt.Sprintf("Контратака: roundEven(урон после DEF × %g); аура: roundEven(d × ATK / %d)", cfg.Factors["counter"], cfg.BaseAttack[0])},
		KnowledgeFact{ID: "berserker", Title: "Берсерк: ранения, лечение, дополнительный удар", Text: "При здоровье цели строго ниже 75% / 50% / 25% основной урон усиливается в 1.10 / 1.20 / 1.35 раза с усечением. Усиление раненой цели учитывается и в контратаке Берсерка после её коэффициента. Классовое лечение ограничено недостающим HP. Дополнительный удар возможен после состоявшейся ближней контратаки, если Берсерк выжил.", Formula: fmt.Sprintf("Лечение = roundEven(фактически снятые основным ударом HP × %g × %g); дополнительный удар = roundEven(основа основного урона × 0.5 × %g)", float64(p.HealPercent)/100, cfg.Factors["heal"], cfg.Factors["followup"])},
		KnowledgeFact{ID: "ranger", Title: "Рейнджер: крит, пробивание и толчок", Text: fmt.Sprintf("Крит работает и в ближнем бою. Дальние попадания накапливают собственное пробивание; третье неблокированное дальнее попадание по живой цели отталкивает её и даёт 1 ОП в пределах общего MP-лимита. Неудачный толчок может вызвать урон столкновения. Порог dodge — %g%%, проверка по целому 0…99: поэтому успешных исходов %d из 100.", 100*cfg.Mechanics["dodge"], int(math.Ceil(100*cfg.Mechanics["dodge"]))), Formula: fmt.Sprintf("Крит: roundEven(d × %g); пробивание за стак: roundEven(%g)", cfg.Factors["crit"], cfg.Factors["armor_break"])},
		KnowledgeFact{ID: "mystic", Title: "Мистик: Overload, Reflex и MP Drain", Text: fmt.Sprintf("Последовательные попадания одной цели: 0 / %d / %d / 0 дополнительного урона до масштаба ATK. На четвёртом и последующих счётчик остаётся на четвёртом. Смена цели и начало личного хода сбрасывают серию. Успешный магический Reflex — не более одного раза за личный ход. MP Drain снимает 1 ОП в начале следующего хода цели; ограничения раунда и уже ожидающего Drain сохраняются.", balance.ResearchRound(float64(p.OverloadSecond)*cfg.Factors["overload"]), balance.ResearchRound(float64(p.OverloadThird)*cfg.Factors["overload"])), Formula: fmt.Sprintf("Reflex = roundEven(%d × %g); объединённая магия = roundEven((Overload + Reflex) × ATK / %d)", p.MysticReflexDamage, cfg.Factors["magic"], cfg.BaseAttack[3])},
		KnowledgeFact{ID: "penetration", Title: "Магия через физический блок", Text: penetration},
		KnowledgeFact{ID: "wards", Title: "Общие защитные эффекты и порядок", Text: "Основной удар: DEF → усиление Берсерка → блок → крит → Ненависть → dodge → focus/сопротивления → одно из glance/surge → opening → close ward → применимый масштаб → буфер → HP. Opening защищает до первого личного хода. Тик ауры обходит opening. Буфер Мистика обновляется перед эффектами его личного хода.", Formula: fmt.Sprintf("glance: u < %g → roundEven(d × 0.55); иначе surge: u < %.4f → roundEven(d × 1.45). Opening: roundEven(d × (1 − %g)); буфер: roundEven(MaxHP × %g).", cfg.Mechanics["glance"], cfg.Mechanics["glance"]+cfg.Mechanics["surge"], cfg.Mechanics["opening"], cfg.Mechanics["arcane"])},
		KnowledgeFact{ID: "resists", Title: "Focus и дополнительные защиты", Text: "Эти эффекты относятся к основной атаке. Положительный удар после close ward не опускается ниже единицы. Нулевые коэффициенты не означают скрытое сопротивление.", Formula: fmt.Sprintf("Дальний focus Ranger: +max(0, roundEven((исходная DEF цели − 8) × %g)); melee ward Guardian: roundEven(d × (1 − %g)); ranged ward Mystic: roundEven(d × (1 − %g)); close ward Mystic: −max(0, roundEven((ATK врага − 17) × %g)).", cfg.Mechanics["ranger_armor_focus"], cfg.Mechanics["guardian_melee_resist"], cfg.Mechanics["mystic_ranged_resist"], cfg.Mechanics["close_ward"])},
	)
}

func KnowledgeCatalog(ctx context.Context) (KnowledgeData, error) {
	ruleset, err := balance.ParseCombatRuleset(os.Getenv("NACHALO_COMBAT_RULESET"))
	if err != nil {
		return KnowledgeData{}, err
	}
	data := KnowledgeData{Ruleset: ruleset, Equipment: []*EquipmentItem{}, Recipes: []CraftRecipe{}, Resources: []KnowledgeResource{}, Monsters: []KnowledgeMonster{}, Drops: []ProfessionLootMonster{}, Gathering: []GatheringCategory{}, Professions: []ProfessionDefinition{}, Artifacts: []KnowledgeArtifact{}, Unavailable: []string{}, Sources: []string{"Серверная конфигурация правил новых матчей", "Каталоги шаблонов предметов и актуальная ruleset-проекция", "Каталоги рецептов, профессий, добычи и сбора", "Публичные определения зданий и рынка"}}
	if ruleset == balance.ExperimentalV34 {
		cfg := balance.ExperimentalV34Config()
		data.Balance = &cfg
	}
	data.Facts = knowledgeFacts(ruleset, data.Balance)
	data.Loot = knowledgeLootRules()
	if data.Balance != nil {
		data.ReflexProgression = map[string]map[string]int{}
	}
	for _, code := range []string{"guardian", "berserker", "ranger", "mystic"} {
		s := ResolveHeroClassStatsForRuleset(code, ruleset)
		m := ResolveHeroClassMeta(code)
		if cfg := data.Balance; cfg != nil {
			rare := 0
			for _, v := range cfg.RareSlotReflex {
				rare += v
			}
			data.ReflexProgression[code] = map[string]int{"naked": s.Agility, "green": s.Agility + cfg.GearMain + cfg.JewelryReflex["green"]["ring"] + cfg.JewelryReflex["green"]["amulet"], "blue": s.Agility + rare + cfg.JewelryReflex["blue"]["ring"] + cfg.JewelryReflex["blue"]["amulet"]}
		}

		data.Heroes = append(data.Heroes, KnowledgeHero{Code: code, Name: m.DisplayName, Image: m.Image, Stats: EquipmentStats{MaxAP: s.MaxAP, MaxMP: s.MaxMP, MaxReactions: s.MaxReactions, MaxHealth: s.MaxHealth, Attack: s.Attack, Defense: s.Defense, Mobility: s.Mobility, Agility: s.Agility, SightRange: s.SightRange, AttackRange: s.AttackRange, IsRanged: s.IsRanged}})
	}
	for _, row := range []struct {
		code, name string
		cfg        baseBuildingConfig
	}{{"guild", "Гильдия ремесленников", guildBuildingConfig}, {"forge", "Кузница", forgeBuildingConfig}, {"library", "Библиотека", libraryBuildingConfig}, {"tavern", "Таверна", tavernBuildingConfig}} {
		data.Buildings = append(data.Buildings, KnowledgeBuilding{Code: row.code, Name: row.name, Costs: map[string]int{"wood": row.cfg.CostWood, "stone": row.cfg.CostStone, "iron": row.cfg.CostIron, "gold": row.cfg.CostGold}})
	}
	data.Dismantle = jewelryCatalog.Dismantle
	data.ItemSources = map[string]string{}
	for _, set := range append(append([]EquipmentCatalogSet{}, equipmentSetsCatalog...), jewelryCatalog.Sets...) {
		for _, item := range set.Items {
			data.ItemSources[item.Code] = set.Source
		}
	}
	failed := func(section string, e error) {
		if e != nil {
			data.Unavailable = append(data.Unavailable, section)
			log.Printf("knowledge %s: %v", section, e)
		}
	}
	if DB == nil {
		data.Unavailable = append(data.Unavailable, "Каталоги БД")
		return data, nil
	}
	pool, poolErr := GetResourcesData()
	failed("Пул ресурсов карты", poolErr)
	for _, resource := range pool {
		if resource.Type != "barrel" && resource.Type != "barbel" {
			data.MapResources = append(data.MapResources, resource.Type)
		}
	}
	data.Recipes, err = PublicCraftRecipes(ctx)
	failed("Рецепты", err)
	data.Drops, err = ProfessionLootCatalog(ctx)
	failed("Профессиональная добыча", err)
	data.Gathering, err = GatheringLootCatalog(ctx)
	failed("Мирный сбор", err)
	rows, err := DB.QueryContext(ctx, `SELECT it.id,it.code,it.name,it.slot,it.item_type,it.handedness,it.rarity,COALESCE(it.class_restriction,''),it.level_requirement,COALESCE(it.image_url,''),it.attack_bonus,it.defense_bonus,it.mobility_bonus,it.agility_bonus,it.max_health_bonus,it.max_energy_bonus,it.sight_range_bonus,it.attack_range_bonus,`+equipmentPocketBonusSQL+`,COALESCE(s.code,''),COALESCE(s.name,'') FROM item_templates it LEFT JOIN item_sets s ON s.id=it.set_id WHERE NOT (`+commonEquipmentSQL("it")+`) ORDER BY it.rarity,it.class_restriction,it.slot,it.id`)
	if err == nil {
		for rows.Next() {
			item := &EquipmentItem{Status: "catalog"}
			err = rows.Scan(&item.TemplateID, &item.Code, &item.Name, &item.Slot, &item.ItemType, &item.Handedness, &item.Rarity, &item.ClassRestriction, &item.LevelRequirement, &item.ImageURL, &item.Bonuses.Attack, &item.Bonuses.Defense, &item.Bonuses.Mobility, &item.Bonuses.Agility, &item.Bonuses.MaxHealth, &item.Bonuses.MaxEnergy, &item.Bonuses.SightRange, &item.Bonuses.AttackRange, &item.Bonuses.PocketBonus, &item.SetCode, &item.SetName)
			if err != nil {
				break
			}
			item.IsTwoHanded = item.Handedness == "two_hand"
			if entry, set, ok := equipmentCatalogItem(item.Code); ok {
				item.Description = entry.Description
				item.CombatBonuses = entry.CombatBonuses
				if item.Slot == "ring" || item.Slot == "amulet" {
					item.JewelrySet = &JewelryPair{Name: set.Name, Bonus: set.PairBonus, Required: 2}
				}
			}
			data.Equipment = append(data.Equipment, item)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
	}
	failed("Экипировка", err)
	rows, err = DB.QueryContext(ctx, `SELECT id,type,COALESCE(description,''),COALESCE(image,''),tradable FROM resources WHERE resource_kind='resource' ORDER BY type,id`)
	if err == nil {
		for rows.Next() {
			var v KnowledgeResource
			err = rows.Scan(&v.ID, &v.Code, &v.Description, &v.Image, &v.Tradable)
			if err != nil {
				break
			}
			v.Name = resourceDisplayName(v.Code)
			data.Resources = append(data.Resources, v)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
	}
	failed("Материалы", err)
	rows, err = DB.QueryContext(ctx, `SELECT id,name,type,max_health,attack,defense,speed,maneuverability,vision,image FROM monsters ORDER BY name,type,id`)
	if err == nil {
		for rows.Next() {
			var v KnowledgeMonster
			err = rows.Scan(&v.ID, &v.Name, &v.Type, &v.HP, &v.Attack, &v.Defense, &v.Speed, &v.Maneuverability, &v.Vision, &v.Image)
			if err != nil {
				break
			}
			v.Image = strings.Trim(v.Image, "\"")
			v.EquipmentChance = data.Loot.UncommonEquipmentChance
			data.Monsters = append(data.Monsters, v)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
	}
	failed("Бестиарий", err)
	rows, err = DB.QueryContext(ctx, `SELECT code,name,description,category FROM profession_catalog WHERE enabled=true ORDER BY category,sort_order,code`)
	if err == nil {
		for rows.Next() {
			var v ProfessionDefinition
			err = rows.Scan(&v.Code, &v.Name, &v.Description, &v.Category)
			if err != nil {
				break
			}
			data.Professions = append(data.Professions, v)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
	}
	failed("Профессии", err)
	rows, err = DB.QueryContext(ctx, `SELECT id,name,COALESCE(description,''),COALESCE(image,''),bonus FROM artifacts ORDER BY id`)
	if err == nil {
		for rows.Next() {
			var v KnowledgeArtifact
			err = rows.Scan(&v.ID, &v.Code, &v.Description, &v.Image, &v.Bonus)
			if err != nil {
				break
			}
			data.Artifacts = append(data.Artifacts, v)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
	}
	failed("Артефакты", err)
	return data, nil
}
