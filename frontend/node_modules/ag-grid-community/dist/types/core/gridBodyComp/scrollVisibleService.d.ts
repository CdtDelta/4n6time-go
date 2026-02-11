import type { NamedBean } from '../context/bean';
import { BeanStub } from '../context/beanStub';
import type { BeanCollection } from '../context/context';
export interface SetScrollsVisibleParams {
    horizontalScrollShowing: boolean;
    verticalScrollShowing: boolean;
}
export declare class ScrollVisibleService extends BeanStub implements NamedBean {
    beanName: "scrollVisibleService";
    private ctrlsService;
    private columnAnimationService;
    private scrollbarWidth;
    wireBeans(beans: BeanCollection): void;
    private horizontalScrollShowing;
    private verticalScrollShowing;
    private horizontalScrollGap;
    private verticalScrollGap;
    postConstruct(): void;
    onDisplayedColumnsChanged(): void;
    private onDisplayedColumnsWidthChanged;
    onCentreViewportResized(): void;
    private updateScrollVisible;
    private updateScrollVisibleImpl;
    private updateScrollGap;
    setScrollsVisible(params: SetScrollsVisibleParams): void;
    isHorizontalScrollShowing(): boolean;
    isVerticalScrollShowing(): boolean;
    hasHorizontalScrollGap(): boolean;
    hasVerticalScrollGap(): boolean;
    getScrollbarWidth(): number;
}
