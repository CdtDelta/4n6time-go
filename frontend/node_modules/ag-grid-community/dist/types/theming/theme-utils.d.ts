export declare const kebabCase: (str: string) => string;
export declare const paramToVariableName: (paramName: string) => string;
export declare const paramToVariableExpression: (paramName: string) => string;
export declare const clamp: (value: number, min: number, max: number) => number;
export declare const logErrorMessage: (message: unknown, error?: unknown) => void;
export declare const proportionToPercent: (value: number) => number;
export declare const camelCase: (str: string) => string;
export declare const memoize: <R, A = void>(fn: (arg: A) => R) => ((arg: A) => R);
