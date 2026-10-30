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

export namespace importer {
	
	export class UnparseableFile {
	    relativePath: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new UnparseableFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.relativePath = source["relativePath"];
	        this.count = source["count"];
	    }
	}
	export class SkippedFile {
	    relativePath: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new SkippedFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.relativePath = source["relativePath"];
	        this.reason = source["reason"];
	    }
	}
	export class ToolStats {
	    fileCount: number;
	    eventCount: number;
	    unparseableCount: number;
	
	    static createFrom(source: any = {}) {
	        return new ToolStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileCount = source["fileCount"];
	        this.eventCount = source["eventCount"];
	        this.unparseableCount = source["unparseableCount"];
	    }
	}
	export class ImportSummary {
	    perTool: Record<string, ToolStats>;
	    skippedFiles: SkippedFile[];
	    unparseableFiles: UnparseableFile[];
	    totalEvents: number;
	    totalFilesProcessed: number;
	    directoriesWalked: number;
	    maxDepthReached: number;
	    toolFamilies: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new ImportSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.perTool = this.convertValues(source["perTool"], ToolStats, true);
	        this.skippedFiles = this.convertValues(source["skippedFiles"], SkippedFile);
	        this.unparseableFiles = this.convertValues(source["unparseableFiles"], UnparseableFile);
	        this.totalEvents = source["totalEvents"];
	        this.totalFilesProcessed = source["totalFilesProcessed"];
	        this.directoriesWalked = source["directoriesWalked"];
	        this.maxDepthReached = source["maxDepthReached"];
	        this.toolFamilies = source["toolFamilies"];
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
	
	

}

export namespace main {
	
	export class DBInfo {
	    path: string;
	    driver: string;
	    eventCount: number;
	    minDate: string;
	    maxDate: string;
	    importedEvents: number;
	    importUnparseable: number;
	    importLogEnabled: boolean;
	    offsetNeedsUpgrade: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DBInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.driver = source["driver"];
	        this.eventCount = source["eventCount"];
	        this.minDate = source["minDate"];
	        this.maxDate = source["maxDate"];
	        this.importedEvents = source["importedEvents"];
	        this.importUnparseable = source["importUnparseable"];
	        this.importLogEnabled = source["importLogEnabled"];
	        this.offsetNeedsUpgrade = source["offsetNeedsUpgrade"];
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
	export class ImportFailure {
	    file: string;
	    path: string;
	    error: string;
	    stage: string;
	    importId: number;
	    eventsAdded: number;
	    backend: string;
	    newDatabase: string;
	    outcome: string;
	    inImportsList: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ImportFailure(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.path = source["path"];
	        this.error = source["error"];
	        this.stage = source["stage"];
	        this.importId = source["importId"];
	        this.eventsAdded = source["eventsAdded"];
	        this.backend = source["backend"];
	        this.newDatabase = source["newDatabase"];
	        this.outcome = source["outcome"];
	        this.inImportsList = source["inImportsList"];
	    }
	}
	export class LoggingStatus {
	    enabled: boolean;
	    filePath: string;
	    persist: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LoggingStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.filePath = source["filePath"];
	        this.persist = source["persist"];
	    }
	}
	export class OpenFailure {
	    backend: string;
	    action: string;
	    target: string;
	    message: string;
	    error: string;
	    stillOpen: string;
	
	    static createFrom(source: any = {}) {
	        return new OpenFailure(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backend = source["backend"];
	        this.action = source["action"];
	        this.target = source["target"];
	        this.message = source["message"];
	        this.error = source["error"];
	        this.stillOpen = source["stillOpen"];
	    }
	}
	export class QueryRequest {
	    filters: FilterItem[];
	    logic: string;
	    orderBy: string;
	    page: number;
	    pageSize: number;
	    searchText: string;
	    searchMode: string;
	    bookmarkOnly: boolean;
	    baseField: string;
	    baseOp: string;
	    baseValue: string;
	    dateFrom: string;
	    dateTo: string;
	
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
	        this.searchMode = source["searchMode"];
	        this.bookmarkOnly = source["bookmarkOnly"];
	        this.baseField = source["baseField"];
	        this.baseOp = source["baseOp"];
	        this.baseValue = source["baseValue"];
	        this.dateFrom = source["dateFrom"];
	        this.dateTo = source["dateTo"];
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
	export class ReadOnlyOpenFailure {
	    path: string;
	    file: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ReadOnlyOpenFailure(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.file = source["file"];
	        this.error = source["error"];
	    }
	}
	export class RecountResult {
	    previous: number;
	    hadPrevious: boolean;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new RecountResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.previous = source["previous"];
	        this.hadPrevious = source["hadPrevious"];
	        this.count = source["count"];
	    }
	}
	export class TimelineBucket {
	    timestamp: string;
	    count: number;
	    notesCount: number;
	
	    static createFrom(source: any = {}) {
	        return new TimelineBucket(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.count = source["count"];
	        this.notesCount = source["notesCount"];
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
	    import_id: number;
	
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
	        this.import_id = source["import_id"];
	    }
	}
	export class ImportRecord {
	    id: number;
	    filename: string;
	    label: string;
	    format: string;
	    imported_at: string;
	    event_count: number;
	    status: string;
	    error_message: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.filename = source["filename"];
	        this.label = source["label"];
	        this.format = source["format"];
	        this.imported_at = source["imported_at"];
	        this.event_count = source["event_count"];
	        this.status = source["status"];
	        this.error_message = source["error_message"];
	    }
	}

}

