import type { NamedBean } from '../../context/bean';
import { BeanStub } from '../../context/beanStub';
import type { UserComponentName } from '../../context/context';
export declare class UserComponentRegistry extends BeanStub implements NamedBean {
    beanName: "userComponentRegistry";
    private agGridDefaults;
    private agGridDefaultParams;
    /** Used to provide useful error messages if a user is trying to use an enterprise component without loading the module. */
    private enterpriseAgDefaultCompsModule;
    private jsComps;
    postConstruct(): void;
    registerDefaultComponent(name: UserComponentName, component: any, params?: any): void;
    private registerJsComponent;
    retrieve(propertyName: string, name: string): {
        componentFromFramework: boolean;
        component: any;
        params?: any;
    } | null;
    private warnAboutMissingComponent;
}
