import type { BeanCollection, ClientSideRowModelStep, IClientSideRowModel, NamedBean, RefreshModelParams, RowBounds, RowDataTransaction, RowModelType, RowNodeTransaction } from 'ag-grid-community';
import { BeanStub, ChangedPath, RowHighlightPosition, RowNode } from 'ag-grid-community';
import { ClientSideNodeManager } from './clientSideNodeManager';
export interface BatchTransactionItem<TData = any> {
    rowDataTransaction: RowDataTransaction<TData>;
    callback: ((res: RowNodeTransaction<TData>) => void) | undefined;
}
export interface RowNodeMap {
    [id: string]: RowNode;
}
export declare class ClientSideRowModel extends BeanStub implements IClientSideRowModel, NamedBean {
    beanName: "rowModel";
    private beans;
    private columnModel;
    private funcColsService;
    private selectionService;
    private valueCache;
    private environment;
    private filterStage;
    private sortStage;
    private flattenStage;
    private groupStage?;
    private aggregationStage?;
    private pivotStage?;
    private filterAggregatesStage?;
    wireBeans(beans: BeanCollection): void;
    private onRowHeightChanged_debounced;
    private rootNode;
    private rowsToDisplay;
    private nodeManager;
    private rowDataTransactionBatch;
    private lastHighlightedRow;
    private applyAsyncTransactionsTimeout;
    /** Has the start method been called */
    private hasStarted;
    /** E.g. data has been set into the node manager already */
    private shouldSkipSettingDataOnStart;
    /**
     * This is to prevent refresh model being called when it's already being called.
     * E.g. the group stage can trigger initial state filter model to be applied. This fires onFilterChanged,
     * which then triggers the listener here that calls refresh model again but at the filter stage
     * (which is about to be run by the original call).
     */
    private isRefreshingModel;
    private rowCountReady;
    postConstruct(): void;
    private addPropertyListeners;
    start(): void;
    private setInitialData;
    ensureRowHeightsValid(startPixel: number, endPixel: number, startLimitIndex: number, endLimitIndex: number): boolean;
    private setRowTopAndRowIndex;
    private clearRowTopAndRowIndex;
    ensureRowsAtPixel(rowNodes: RowNode[], pixel: number, increment?: number): boolean;
    highlightRowAtPixel(rowNode: RowNode | null, pixel?: number): void;
    getHighlightPosition(pixel: number, rowNode?: RowNode): RowHighlightPosition;
    getLastHighlightedRowNode(): RowNode | null;
    private isHighlightingCurrentPosition;
    private clearHighlightedRow;
    isLastRowIndexKnown(): boolean;
    getRowCount(): number;
    /**
     * Returns the number of rows with level === 1
     */
    getTopLevelRowCount(): number;
    /**
     * Get the row display index by the top level index
     * top level index is the index of rows with level === 1
     */
    getTopLevelRowDisplayedIndex(topLevelIndex: number): number;
    getRowBounds(index: number): RowBounds | null;
    onRowGroupOpened(): void;
    private onFilterChanged;
    private onSortChanged;
    getType(): RowModelType;
    private onValueChanged;
    private createChangePath;
    private isSuppressModelUpdateAfterUpdateTransaction;
    private buildRefreshModelParams;
    refreshModel(paramsOrStep: RefreshModelParams | ClientSideRowModelStep | undefined): void;
    isEmpty(): boolean;
    isRowsToRender(): boolean;
    getNodesInRangeForSelection(firstInRange: RowNode, lastInRange: RowNode): RowNode[];
    setDatasource(datasource: any): void;
    getTopLevelNodes(): RowNode[] | null;
    getRootNode(): RowNode;
    getRow(index: number): RowNode;
    isRowPresent(rowNode: RowNode): boolean;
    getRowIndexAtPixel(pixelToMatch: number): number;
    private isRowInPixel;
    forEachLeafNode(callback: (node: RowNode, index: number) => void): void;
    forEachNode(callback: (node: RowNode, index: number) => void, includeFooterNodes?: boolean): void;
    forEachNodeAfterFilter(callback: (node: RowNode, index: number) => void, includeFooterNodes?: boolean): void;
    forEachNodeAfterFilterAndSort(callback: (node: RowNode, index: number) => void, includeFooterNodes?: boolean): void;
    forEachPivotNode(callback: (node: RowNode, index: number) => void, includeFooterNodes?: boolean): void;
    private recursivelyWalkNodesAndCallback;
    doAggregate(changedPath?: ChangedPath): void;
    private doFilterAggregates;
    expandOrCollapseAll(expand: boolean): void;
    private doSort;
    private doRowGrouping;
    private doFilter;
    private doPivot;
    getNodeManager(): ClientSideNodeManager;
    getRowNode(id: string): RowNode | undefined;
    setRowData(rowData: any[]): void;
    private dispatchUpdateEventsAndRefresh;
    batchUpdateRowData(rowDataTransaction: RowDataTransaction, callback?: (res: RowNodeTransaction) => void): void;
    flushAsyncTransactions(): void;
    private executeBatchUpdateRowData;
    /**
     * Used to apply transaction changes.
     * Called by gridApi & rowDragFeature
     */
    updateRowData(rowDataTran: RowDataTransaction): RowNodeTransaction | null;
    /**
     * Used to apply generated transaction
     */
    afterImmutableDataChange(rowNodeTransaction: RowNodeTransaction, rowNodesOrderChanged: boolean): void;
    /**
     * Common to:
     * - executeBatchUpdateRowData (batch transactions)
     * - updateRowData (single transaction)
     * - afterImmutableDataChange (generated transaction)
     *
     * @param rowNodeTrans - the transactions to apply
     * @param orderChanged - whether the order of the rows has changed, either via generated transaction or user provided addIndex
     */
    private commonUpdateRowData;
    private doRowsToDisplay;
    onRowHeightChanged(): void;
    /** This method is debounced. It is used for row auto-height. If we don't debounce,
     * then the Row Models will end up recalculating each row position
     * for each row height change and result in the Row Renderer laying out rows.
     * This is particularly bad if using print layout, and showing eg 1,000 rows,
     * each row will change it's height, causing Row Model to update 1,000 times.
     */
    onRowHeightChangedDebounced(): void;
    resetRowHeights(): void;
    private resetRowHeightsForAllRowNodes;
    private onGridStylesChanges;
    private onGridReady;
    isRowDataLoaded(): boolean;
    destroy(): void;
}
