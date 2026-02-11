import type { AgColumn } from '../entities/agColumn';
import type { GridOptionsService } from '../gridOptionsService';
export declare const iconNameClassMap: {
    readonly columnGroupOpened: "expanded";
    readonly columnGroupClosed: "contracted";
    readonly columnSelectClosed: "tree-closed";
    readonly columnSelectOpen: "tree-open";
    readonly columnSelectIndeterminate: "tree-indeterminate";
    readonly columnMovePin: "pin";
    readonly columnMoveHide: "eye-slash";
    readonly columnMoveMove: "arrows";
    readonly columnMoveLeft: "left";
    readonly columnMoveRight: "right";
    readonly columnMoveGroup: "group";
    readonly columnMoveValue: "aggregation";
    readonly columnMovePivot: "pivot";
    readonly dropNotAllowed: "not-allowed";
    readonly groupContracted: "tree-closed";
    readonly groupExpanded: "tree-open";
    readonly setFilterGroupClosed: "tree-closed";
    readonly setFilterGroupOpen: "tree-open";
    readonly setFilterGroupIndeterminate: "tree-indeterminate";
    readonly chart: "chart";
    readonly close: "cross";
    readonly cancel: "cancel";
    readonly check: "tick";
    readonly first: "first";
    readonly previous: "previous";
    readonly next: "next";
    readonly last: "last";
    readonly linked: "linked";
    readonly unlinked: "unlinked";
    readonly colorPicker: "color-picker";
    readonly groupLoading: "loading";
    readonly menu: "menu";
    readonly menuAlt: "menu-alt";
    readonly filter: "filter";
    readonly columns: "columns";
    readonly maximize: "maximize";
    readonly minimize: "minimize";
    readonly menuPin: "pin";
    readonly menuValue: "aggregation";
    readonly menuAddRowGroup: "group";
    readonly menuRemoveRowGroup: "group";
    readonly clipboardCopy: "copy";
    readonly clipboardCut: "cut";
    readonly clipboardPaste: "paste";
    readonly pivotPanel: "pivot";
    readonly rowGroupPanel: "group";
    readonly valuePanel: "aggregation";
    readonly columnDrag: "grip";
    readonly rowDrag: "grip";
    readonly save: "save";
    readonly csvExport: "csv";
    readonly excelExport: "excel";
    readonly smallDown: "small-down";
    readonly smallLeft: "small-left";
    readonly smallRight: "small-right";
    readonly smallUp: "small-up";
    readonly sortAscending: "asc";
    readonly sortDescending: "desc";
    readonly sortUnSort: "none";
    readonly advancedFilterBuilder: "group";
    readonly advancedFilterBuilderDrag: "grip";
    readonly advancedFilterBuilderInvalid: "not-allowed";
    readonly advancedFilterBuilderMoveUp: "up";
    readonly advancedFilterBuilderMoveDown: "down";
    readonly advancedFilterBuilderAdd: "plus";
    readonly advancedFilterBuilderRemove: "minus";
    readonly chartsMenuEdit: "chart";
    readonly chartsMenuAdvancedSettings: "settings";
    readonly chartsMenuAdd: "plus";
    readonly checkboxChecked: "checkbox-checked";
    readonly checkboxIndeterminate: "checkbox-indeterminate";
    readonly checkboxUnchecked: "checkbox-unchecked";
    readonly radioButtonOn: "radio-button-on";
    readonly radioButtonOff: "radio-button-off";
};
export type IconName = keyof typeof iconNameClassMap;
export type IconValue = (typeof iconNameClassMap)[IconName];
/**
 * If icon provided, use this (either a string, or a function callback).
 * if not, then use the default icon from the theme
 * @param {string} iconName
 * @param {GridOptionsService} gos
 * @param {Column | null} [column]
 * @returns {Element}
 */
export declare function _createIcon(iconName: string, gos: GridOptionsService, column: AgColumn | null): Element;
export declare function _createIconNoSpan(iconName: string, gos: GridOptionsService, column?: AgColumn | null, forceCreate?: boolean): Element | undefined;
