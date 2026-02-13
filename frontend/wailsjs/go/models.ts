export namespace database {
	
	export class SavedQuery {
	    Name: string;
	    Query: string;
	
	    static createFrom(source: any = {}) {
	        return new SavedQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Query = source["Query"];
	    }
	}

}

export namespace main {
	
	export class DBInfo {
	    path: string;
	    eventCount: number;
	    minDate: string;
	    maxDate: string;
	
	    static createFrom(source: any = {}) {
	        return new DBInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.eventCount = source["eventCount"];
	        this.minDate = source["minDate"];
	        this.maxDate = source["maxDate"];
	    }
	}
	export class FilterItem {
	    field: string;
	    operator: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new FilterItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.field = source["field"];
	        this.operator = source["operator"];
	        this.value = source["value"];
	    }
	}
	export class QueryRequest {
	    filters: FilterItem[];
	    logic: string;
	    orderBy: string;
	    page: number;
	    pageSize: number;
	    searchText: string;
	    bookmarkOnly: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QueryRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filters = this.convertValues(source["filters"], FilterItem);
	        this.logic = source["logic"];
	        this.orderBy = source["orderBy"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.searchText = source["searchText"];
	        this.bookmarkOnly = source["bookmarkOnly"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class QueryResponse {
	    events: model.Event[];
	    totalCount: number;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new QueryResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.events = this.convertValues(source["events"], model.Event);
	        this.totalCount = source["totalCount"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TimelineBucket {
	    timestamp: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new TimelineBucket(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.count = source["count"];
	    }
	}

}

export namespace model {
	
	export class Event {
	    id: number;
	    timezone: string;
	    macb: string;
	    source: string;
	    sourcetype: string;
	    type: string;
	    user: string;
	    host: string;
	    desc: string;
	    filename: string;
	    inode: string;
	    notes: string;
	    format: string;
	    extra: string;
	    datetime: string;
	    reportnotes: string;
	    inreport: string;
	    tag: string;
	    color: string;
	    offset: number;
	    store_number: number;
	    store_index: number;
	    vss_store_number: number;
	    url: string;
	    record_number: string;
	    event_identifier: string;
	    event_type: string;
	    source_name: string;
	    user_sid: string;
	    computer_name: string;
	    bookmark: number;
	
	    static createFrom(source: any = {}) {
	        return new Event(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timezone = source["timezone"];
	        this.macb = source["macb"];
	        this.source = source["source"];
	        this.sourcetype = source["sourcetype"];
	        this.type = source["type"];
	        this.user = source["user"];
	        this.host = source["host"];
	        this.desc = source["desc"];
	        this.filename = source["filename"];
	        this.inode = source["inode"];
	        this.notes = source["notes"];
	        this.format = source["format"];
	        this.extra = source["extra"];
	        this.datetime = source["datetime"];
	        this.reportnotes = source["reportnotes"];
	        this.inreport = source["inreport"];
	        this.tag = source["tag"];
	        this.color = source["color"];
	        this.offset = source["offset"];
	        this.store_number = source["store_number"];
	        this.store_index = source["store_index"];
	        this.vss_store_number = source["vss_store_number"];
	        this.url = source["url"];
	        this.record_number = source["record_number"];
	        this.event_identifier = source["event_identifier"];
	        this.event_type = source["event_type"];
	        this.source_name = source["source_name"];
	        this.user_sid = source["user_sid"];
	        this.computer_name = source["computer_name"];
	        this.bookmark = source["bookmark"];
	    }
	}

}

