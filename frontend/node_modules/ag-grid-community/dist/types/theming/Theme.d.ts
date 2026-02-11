import { type GridTheme } from 'ag-grid-community';
import type { Part } from './Part';
import { type CoreParams } from './styles/core/core-css';
export type Theme<TParams = unknown> = GridTheme & {
    readonly id: string;
    readonly dependencies: readonly Part[];
    readonly defaults: Partial<TParams>;
    /**
     * Return a new theme that uses an theme part. The part will replace any
     * existing part of the same feature
     *
     * @param part a part, or a no-arg function that returns a part
     */
    withPart<TPartParams>(part: Part<TPartParams> | (() => Part<TPartParams>)): Theme<TParams & TPartParams>;
    /**
     * Return a new theme with different default values for the specified
     * params.
     *
     * @param defaults an object containing params e.g. {spacing: 10}
     */
    withParams(defaults: Partial<TParams>): Theme<TParams>;
    /**
     * Return the complete rendered CSS for this theme. This can be used at
     * build time to generate CSS if your application requires that CSS be
     * served from a static file.
     */
    getCSS(): string;
    /**
     * Return the params used to render the theme, including the default values
     * and any overrides supplied by parts and `withParams` calls.
     */
    getParams(): Record<string, unknown>;
};
/**
 * Create a custom theme containing core grid styles but no parts.
 *
 * @param id an optional identifier for debugging, if omitted one will be generated
 */
export declare const createTheme: (id?: string) => Theme<CoreParams>;
