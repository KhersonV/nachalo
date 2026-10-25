export type RawInventoryItem = {
  item_type: "resource" | "artifact" | "scroll";
  item_id: number;
  name: string;
  item_count: number;
  image: string;
  description: string;
  origin?: string;
  resource_code?: string;
  consumable_effect?: string;
  bonus?: Record<string, number>;
  effect?: Record<string, number>;
  inventory_key?: string;
};
