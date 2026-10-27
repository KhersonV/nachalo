import { API_BASE } from '@/utils/serviceUrls';
import type { CodexData } from './types';

export async function loadKnowledge(token: string, signal: AbortSignal): Promise<CodexData> {
 const response = await fetch(`${API_BASE}/game/knowledge`, { headers: { Authorization: `Bearer ${token}` }, signal, cache: 'no-store' });
 if (!response.ok) throw new Error(response.status === 401 ? 'Сессия истекла. Войдите в игру заново.' : 'Хроники сейчас недоступны. Повторите загрузку.');
 const raw = await response.json() as CodexData;
 if (!raw || !Array.isArray(raw.heroes) || !Array.isArray(raw.facts) || !['legacy', 'experimental_v34'].includes(raw.ruleset)) throw new Error('Не удалось прочитать данные Хроник.');
 for (const key of ['equipment','recipes','resources','mapResources','monsters','drops','gathering','professions','artifacts','buildings','shop','unavailable','sources'] as const) {
  if (!Array.isArray(raw[key])) Object.assign(raw, { [key]: [] });
 }
 // Apply only to equipment; Common resources, scrolls and materials remain.
 raw.equipment = raw.equipment.filter(item => !['gray','grey','common'].includes(item.rarity.toLowerCase()));
 raw.recipes = raw.recipes.filter(recipe => !['equipment','jewelry'].includes(recipe.category) || !['gray','grey','common'].includes((recipe.rarity || '').toLowerCase()));
 raw.dismantle ??= {};
 raw.itemSources ??= {};
 return raw;
}
