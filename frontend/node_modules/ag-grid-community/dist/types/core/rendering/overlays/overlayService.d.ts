import type { NamedBean } from '../../context/bean';
import { BeanStub } from '../../context/beanStub';
import type { BeanCollection } from '../../context/context';
import type { OverlayWrapperComponent } from './overlayWrapperComponent';
export declare class OverlayService extends BeanStub implements NamedBean {
    beanName: "overlayService";
    private userComponentFactory;
    private rowModel;
    private ctrlsService;
    private isClientSide;
    private columnModel;
    private state;
    private showInitialOverlay;
    private exclusive?;
    private wrapperPadding;
    wireBeans(beans: BeanCollection): void;
    private overlayWrapperComp;
    postConstruct(): void;
    setOverlayWrapperComp(overlayWrapperComp: OverlayWrapperComponent | undefined): void;
    /** Returns true if the overlay is visible. */
    isVisible(): boolean;
    /** Returns true if the overlay is visible and is exclusive (popup over the grid) */
    isExclusive(): boolean;
    /** Gets the overlay wrapper component */
    getOverlayWrapper(): OverlayWrapperComponent | undefined;
    showLoadingOverlay(): void;
    showNoRowsOverlay(): void;
    hideOverlay(): void;
    private updateOverlayVisibility;
    private doShowLoadingOverlay;
    private doShowNoRowsOverlay;
    private doHideOverlay;
    private showOverlay;
    private updateExclusive;
    private onGridSizeChanged;
    private refreshWrapperPadding;
}
