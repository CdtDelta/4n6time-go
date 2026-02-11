import type { NamedBean } from '../context/bean';
import { BeanStub } from '../context/beanStub';
import { AgColumn } from '../entities/agColumn';
export interface IControlsColService {
    createControlsCols(): AgColumn[];
    isControlsColEnabled(): boolean;
}
export declare const CONTROLS_COLUMN_ID_PREFIX: "ag-Grid-ControlsColumn";
export declare class ControlsColService extends BeanStub implements NamedBean, IControlsColService {
    beanName: "controlsColService";
    isControlsColEnabled(): boolean;
    createControlsCols(): AgColumn[];
}
