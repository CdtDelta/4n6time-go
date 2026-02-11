import type { BorderValue, ColorValue, LengthValue, ShadowValue } from '../../../theme-types';
export type InputStyleParams = {
    /**
     * Background color for text inputs
     */
    inputBackgroundColor: ColorValue;
    /**
     * Border around text inputs (or underneath, if using the underlined input style)
     */
    inputBorder: BorderValue;
    /**
     * Corner radius of text inputs
     */
    inputBorderRadius: LengthValue;
    /**
     * Background color for disabled text inputs
     */
    inputDisabledBackgroundColor: ColorValue;
    /**
     * Border around disabled text inputs (or underneath, if using the underlined input style)
     */
    inputDisabledBorder: BorderValue;
    /**
     * Color of text within disabled text inputs
     */
    inputDisabledTextColor: ColorValue;
    /**
     * Background color for focussed text inputs
     */
    inputFocusBackgroundColor: ColorValue;
    /**
     * Border around focussed text inputs (or underneath, if using the underlined input style)
     */
    inputFocusBorder: BorderValue;
    /**
     * Shadow around focussed text inputs
     */
    inputFocusShadow: ShadowValue;
    /**
     * Color of text within focussed text inputs
     */
    inputFocusTextColor: ColorValue;
    /**
     * Minimum height of text inputs
     */
    inputHeight: LengthValue;
    /**
     * Background color for text inputs in an invalid state
     */
    inputInvalidBackgroundColor: ColorValue;
    /**
     * Border around text inputs in an invalid state (or underneath, if using the underlined input style)
     */
    inputInvalidBorder: BorderValue;
    /**
     * Color of text within text inputs in an invalid state
     */
    inputInvalidTextColor: ColorValue;
    /**
     * Padding at the start of text in text inputs
     */
    inputPaddingStart: LengthValue;
    /**
     * Color of text within text inputs
     */
    inputTextColor: ColorValue;
};
export declare const inputStyleBase: import("../../../Part").Part<import("../../core/core-css").CoreParams & InputStyleParams>;
export declare const inputStyleBordered: import("../../../Part").Part<import("../../core/core-css").CoreParams & InputStyleParams>;
export declare const inputStyleUnderlined: import("../../../Part").Part<import("../../core/core-css").CoreParams & InputStyleParams>;
