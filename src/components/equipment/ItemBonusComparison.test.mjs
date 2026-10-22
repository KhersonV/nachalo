import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import ts from 'typescript';
import { compareItemBonuses } from '../../features/equipment/equipmentView.ts';

const require = createRequire(import.meta.url);
const source = readFileSync(new URL('./ItemBonusComparison.tsx', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX } }).outputText;
const cssClasses = new Proxy({}, { get: (_, key) => key === '__esModule' ? false : String(key) });
const componentModule = { exports: {} };
const componentRequire = (name) => name === 'react/jsx-runtime' ? require(name) : name.endsWith('EquipmentPage.module.css') ? { default: cssClasses } : require(name);
new Function('require', 'module', 'exports', compiled)(componentRequire, componentModule, componentModule.exports);
const ItemBonusComparison = componentModule.exports.default;

test('comparison component renders item values, improvements, and downgrades', () => {
    const current = { bonuses: { attack: 2, defense: 3 } };
    const selected = { bonuses: { attack: 5, defense: 1 } };
    const html = renderToStaticMarkup(React.createElement(ItemBonusComparison, { rows: compareItemBonuses(current, selected) }));
    assert.match(html, /Attack/);
    assert.match(html, /Defense/);
    assert.match(html, /class="positive">\+3/);
    assert.match(html, /class="negative">-2/);
});

test('comparison component has an honest empty state', () => {
    const html = renderToStaticMarkup(React.createElement(ItemBonusComparison, { rows: [] }));
    assert.match(html, /No item stat bonuses to compare/);
});
