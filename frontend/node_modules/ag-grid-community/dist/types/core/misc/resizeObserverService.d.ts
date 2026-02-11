import type { NamedBean } from '../context/bean';
import { BeanStub } from '../context/beanStub';
export declare class ResizeObserverService extends BeanStub implements NamedBean {
    beanName: "resizeObserverService";
    observeResize(element: HTMLElement, callback: () => void): () => void;
}
