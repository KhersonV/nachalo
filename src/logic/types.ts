// src/logic/types.ts

// Определяем возможные игровые режимы.
export type GameMode = "PVE" | "1v1" | "3v3" | "5v5";

// Тип для описания состояния монстра.
export type MonsterState = {
  id: number; // Уникальный идентификатор монстра.
  name: string; // Имя монстра (например, "Orc").
  type: "aggressive" | "neutral"; // Тип поведения монстра: агрессивный или нейтральный.
  hp: number; // Текущее здоровье монстра.
  maxHp: number; // Максимальное здоровье монстра.
  attack: number; // Сила атаки монстра.
  defense: number; // Значение защиты монстра.
  vision: number; // Дальность обзора монстра (количество клеток).
  image: Record<string, string>; // Изображения монстра в разных состояниях (например, иконка или полный арт).
};

// Тип для описания ресурса на клетке.
export type ResourceType = {
  type: string; // Тип ресурса (например, "wood", "iron").
  difficulty: number; // Уровень сложности добычи ресурса.
  image: Record<string, string>; // Изображения ресурса для разных состояний или интерфейса.
  description: string; // Описание ресурса.
  rarity: number; // Редкость ресурса (например, от 1 до 10).
  effect: number; // Эффект от использования ресурса (например, +10 к здоровью).
  terrains: string[]; // Типы местности, где этот ресурс может находиться (например, ["forest", "mountain"]).
};

// Типы действий, которые может выполнять игрок.
export type PlayerAction =
  | "move" // Перемещение.
  | "attack" // Атака.
  | "collect" // Сбор ресурсов.
  | "useItem" // Использование предметов.
  | "interact" // Взаимодействие с объектами (например, открытие бочки).
  | "passTurn" // Передача хода.
  | "pickArtifact" // Поднятие артефакта.
  | "loseArtifact"; // Потеря артефакта.

// Способности игрока, определяющие его возможности в игре.
export type PlayerAbilities = {
  canMove: boolean; // Может ли игрок двигаться.
  canAttack: boolean; // Может ли игрок атаковать.
  canCollectResources: boolean; // Может ли игрок собирать ресурсы.
  canUseItems: boolean; // Может ли игрок использовать предметы.
  canInteractWithObjects: boolean; // Может ли игрок взаимодействовать с объектами.
  canPassTurn: boolean; // Может ли игрок передавать ход.
  canPickArtifact: boolean; // Может ли игрок поднимать артефакты.
  canLoseArtifact: boolean; // Может ли игрок терять артефакты.
};

// Тип для описания переходной клетки (например, портал или лестница).
export type TransitionTile = {
  from: string; // Откуда происходит переход (например, "forest").
  to: string; // Куда происходит переход (например, "cave").
  image: string; // Изображение для отображения перехода.
};

// Дополнительные бонусные атрибуты, которые могут быть у предметов или артефактов.
type BonusAttributes = {
  energy?: number; // Бонус к энергии.
  maxEnergy?: number; // Бонус к максимальной энергии.
  visionRange?: number; // Бонус к дальности обзора.
  health?: number; // Бонус к здоровью.
  maxHealth?: number; // Бонус к максимальному здоровью.
  attack?: number; // Бонус к атаке.
  defense?: number; // Бонус к защите.
};

// Тип для описания предметов в инвентаре.
type InventoryItem = {
  count: number; // Количество данного типа предметов.
  image: string; // Изображение предмета.
  description: string; // Описание предмета.
  bonus?: BonusAttributes; // Бонусы, которые даёт предмет.
};

// Тип для инвентаря игрока (ключ — тип ресурса, значение — данные об этом ресурсе).
type Inventory = Record<string, InventoryItem>;

// Тип для описания состояния игрока.
export type PlayerState = {
  id: number; // Уникальный идентификатор игрока.
  name: string; // Имя игрока.
  position: { x: number; y: number }; // Позиция игрока на карте.
  energy: number; // Текущая энергия игрока.
  maxEnergy: number; // Максимальная энергия игрока.
  level: number; // Уровень игрока.
  expirience: number; // Текущий опыт игрока.
  max_expirience: number; // Максимальный опыт для перехода на следующий уровень.
  visionRange: number; // Дальность обзора игрока.
  health: number; // Текущее здоровье игрока.
  maxHealth: number; // Максимальное здоровье игрока.
  attack: number; // Значение атаки игрока.
  defense: number; // Значение защиты игрока.
  image: string; // Изображение игрока.
  inventory: Inventory; // Инвентарь игрока.
  abilities: PlayerAbilities; // Способности игрока.
};

// Тип для описания клетки карты.
export type Cell = {
  id: number; // Уникальный идентификатор клетки.
  x: number; // Координата X клетки.
  y: number; // Координата Y клетки.
  terrain: string; // Тип местности (например, "forest", "mountain").
  resource: ResourceType | null; // Ресурс, находящийся на клетке (если есть).
  isPortal?: boolean; // Флаг, указывающий, является ли клетка порталом.
  monster?: MonsterState; // Монстр, находящийся на клетке (если есть).
  isTransition?: boolean; // Флаг, указывающий, является ли клетка переходной.
  transition?: TransitionTile; // Данные о переходе (если клетка переходная).
};

// Тип для глобального состояния игры.
export type GameState = {
  mode: GameMode; // Текущий игровой режим.
  players: PlayerState[]; // Массив игроков.
  grid: Cell[] | null; // Сетка клеток карты (или null, если карта ещё не сгенерирована).
  mapWidth: number; // Ширина карты (в клетках).
  mapHeight: number; // Высота карты (в клетках).
  artifactOwner: number | null; // ID игрока, владеющего артефактом (или null, если артефакт не поднят).
  portalPosition: { x: number; y: number } | null; // Позиция портала на карте.
  instanceId: string; // Уникальный идентификатор текущей игровой сессии.
  currentPlayerIndex: number; // Индекс текущего игрока (для очередности ходов).
  turnCycle: number; // Номер текущего круга игры.
};
