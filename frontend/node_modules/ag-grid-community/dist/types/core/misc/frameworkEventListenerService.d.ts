import type { AgEventType } from '../eventTypes';
import type { ColumnEventName } from '../interfaces/iColumn';
import type { IFrameworkOverrides } from '../interfaces/iFrameworkOverrides';
import type { RowNodeEventType } from '../interfaces/iRowNode';
type EventTypeToWrap = AgEventType | RowNodeEventType | ColumnEventName;
export declare class FrameworkEventListenerService<TEventListener extends (e: any) => void, TGlobalEventListener extends (name: string, e: any) => void> {
    private frameworkOverrides;
    private wrappedListeners;
    private wrappedGlobalListeners;
    constructor(frameworkOverrides: IFrameworkOverrides);
    wrap(eventType: EventTypeToWrap, userListener: TEventListener): TEventListener;
    wrapGlobal(userListener: TGlobalEventListener): TGlobalEventListener;
    unwrap(eventType: EventTypeToWrap, userListener: TEventListener): TEventListener;
    unwrapGlobal(userListener: TGlobalEventListener): TGlobalEventListener;
}
export {};
