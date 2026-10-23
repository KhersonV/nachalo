import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import {
    HERO_ARTWORK_FALLBACK,
    getHeroArtwork,
    getHeroFullbody,
    getHeroGameImage,
    getHeroPortrait,
    normalizeHeroClass,
} from './heroArtwork.ts';

const classes = ['guardian', 'ranger', 'berserker', 'mystic'];
const assetFile = (url) => join(process.cwd(), 'public', url.slice(1));

test('every class resolves distinct portrait, fullbody, and game image', () => {
    for (const heroClass of classes) {
        const entry = getHeroArtwork(heroClass);
        assert.equal(getHeroPortrait(heroClass), entry.portrait);
        assert.equal(getHeroFullbody(heroClass), entry.fullbody);
        assert.equal(getHeroGameImage(heroClass), entry.gameImage);
        for (const key of ['portrait', 'fullbody']) {
            const path = assetFile(entry[key]);
            assert.ok(statSync(path).size > 0, `${heroClass} ${key} exists`);
            const header = readFileSync(path).subarray(0, 12);
            assert.equal(header.toString('ascii', 0, 4), 'RIFF');
            assert.equal(header.toString('ascii', 8, 12), 'WEBP');
        }
    }
    assert.equal(new Set(classes.map(heroClass => getHeroPortrait(heroClass))).size, 4);
    assert.equal(new Set(classes.map(heroClass => getHeroFullbody(heroClass))).size, 4);
});

test('aliases and case normalize without guessing numeric character IDs', () => {
    assert.equal(normalizeHeroClass(' BERSERK '), 'berserker');
    assert.equal(normalizeHeroClass('Mag'), 'mystic');
    assert.equal(normalizeHeroClass('RaNgEr'), 'ranger');
    assert.equal(normalizeHeroClass(12), null);
    assert.equal(normalizeHeroClass('12'), null);
    assert.equal(normalizeHeroClass('constructor'), null);
    assert.equal(getHeroPortrait(12), HERO_ARTWORK_FALLBACK);
    assert.equal(getHeroFullbody('unknown'), HERO_ARTWORK_FALLBACK);
});

test('individual hero image keeps priority over generic class metadata', () => {
    assert.equal(getHeroPortrait('guardian', '/guardian/guardian.webp'), getHeroArtwork('guardian').portrait);
    assert.equal(getHeroPortrait('guardian', '/assets/ui/heroes/guardian.webp'), getHeroArtwork('guardian').portrait);
    assert.equal(getHeroPortrait('guardian', '/skins/player-guardian.webp'), '/skins/player-guardian.webp');
});
