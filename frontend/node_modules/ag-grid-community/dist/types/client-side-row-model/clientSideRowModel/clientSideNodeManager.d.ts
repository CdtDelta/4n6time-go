import type { BeanCollection, EventService, FuncColsService, GridOptionsService, ISelectionService, RowDataTransaction, RowNodeTransaction } from 'ag-grid-community';
import { RowNode } from 'ag-grid-community';
/** Result of ClientSideNodeManager.updateRowData method */
export interface ClientSideNodeManagerUpdateRowDataResult {
    /** The RowNodeTransaction containing all the removals, updates and additions */
    rowNodeTransaction: RowNodeTransaction;
    /** True if at least one row was inserted (and not just appended) */
    rowsInserted: boolean;
}
export declare class ClientSideNodeManager {
    private readonly rootNode;
    private gos;
    private eventService;
    private funcColsService;
    private selectionService;
    private beans;
    private nextId;
    private rowCountReady;
    private allNodesMap;
    constructor(rootNode: RowNode, gos: GridOptionsService, eventService: EventService, funcColsService: FuncColsService, selectionService: ISelectionService, beans: BeanCollection);
    getCopyOfNodesMap(): {
        [id: string]: RowNode;
    };
    getRowNode(id: string): RowNode | undefined;
    setRowData(rowData: any[]): RowNode[] | undefined;
    updateRowData(rowDataTran: RowDataTransaction): ClientSideNodeManagerUpdateRowDataResult;
    /**
     * Used by the immutable service, after updateRowData, after updating with a generated transaction to
     * apply the order as specified by the the new data. We use sourceRowIndex to determine the order of the rows.
     * Time complexity is O(n) where n is the number of rows/rowData
     * @returns true if the order changed, otherwise false
     */
    updateRowOrderFromRowData<TData>(rowData: TData[]): boolean;
    isRowCountReady(): boolean;
    private dispatchRowDataUpdateStartedEvent;
    private updateSelection;
    private executeAdd;
    private sanitizeAddIndex;
    private executeRemove;
    private executeUpdate;
    private lookupRowNode;
    private createNode;
    private setMasterForRow;
    private isExpanded;
}
