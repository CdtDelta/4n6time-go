import type { CoreParams } from './styles/core/core-css';
import type { Feature } from './theme-types';
import { type CssFragment } from './theme-types';
export type Part<TParams = unknown> = {
    readonly id: string;
    readonly feature: string;
    readonly variant: string;
    readonly defaults: Partial<TParams>;
    readonly css: ReadonlyArray<CssFragment>;
    /**
     * Return a new Part new different default values for core grid params.
     */
    withParams(defaults: Partial<TParams>): Part<TParams>;
    /**
     * Return a new part with additional params. Unlike `withParams`, this can
     * be used to add support for params used by the part's own CSS.
     */
    withAdditionalParams<TAdditionalParams>(defaults: TAdditionalParams): Part<TParams & TAdditionalParams>;
    /**
     * Return a new Part with additional CSS.
     */
    withCSS(css: CssFragment): Part<TParams>;
};
export declare const createPartVariant: <T>(part: Part<T>, variant: string) => Part<T>;
type AnyString = string & {};
/**
 * Create a new empty part.
 *
 * @param feature an The part feature, e.g. 'iconSet'. Adding a part to a theme will remove any existing part with the same feature.
 * @param variant an optional identifier for debugging, if omitted one will be generated
 */
export declare const createPart: (feature: Feature | AnyString, variant?: string) => Part<CoreParams>;
export {};
