import type { compareItemBonuses } from "@/features/equipment/equipmentView";
import styles from "@/styles/EquipmentPage.module.css";

type ComparisonRow = ReturnType<typeof compareItemBonuses>[number];

const signed = (value: number) => value > 0 ? `+${value}` : String(value);

export default function ItemBonusComparison({ rows }: { rows: ComparisonRow[] }) {
    return <div className={styles.comparisonRows} aria-label="Item bonus comparison">
        <div className={styles.comparisonLegend}><span>Item bonus</span><span>Now</span><span>Selected</span><span>Difference</span></div>
        {rows.map(row => <div key={row.key} className={styles.comparisonRow}>
            <span>{row.label}</span>
            <span>{signed(row.currentValue)}</span>
            <span>{signed(row.selectedValue)}</span>
            <strong className={row.difference > 0 ? styles.positive : row.difference < 0 ? styles.negative : ""}>{row.difference === 0 ? "—" : signed(row.difference)}</strong>
        </div>)}
        {rows.length === 0 && <p className={styles.emptyBonus}>No item stat bonuses to compare.</p>}
    </div>;
}
