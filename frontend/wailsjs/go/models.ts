export namespace main {
	
	export class FolderView {
	    path: string;
	    backup: boolean;
	    sync: boolean;
	    lastBackupAt: string;
	
	    static createFrom(source: any = {}) {
	        return new FolderView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.backup = source["backup"];
	        this.sync = source["sync"];
	        this.lastBackupAt = source["lastBackupAt"];
	    }
	}
	export class NodeView {
	    name: string;
	    type: string;
	    path: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new NodeView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.path = source["path"];
	        this.size = source["size"];
	    }
	}
	export class SnapshotView {
	    id: string;
	    shortId: string;
	    time: string;
	    paths: string[];
	
	    static createFrom(source: any = {}) {
	        return new SnapshotView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.shortId = source["shortId"];
	        this.time = source["time"];
	        this.paths = source["paths"];
	    }
	}
	export class StatusView {
	    configured: boolean;
	    user: string;
	    bucket: string;
	    endpoint: string;
	    folders: FolderView[];
	
	    static createFrom(source: any = {}) {
	        return new StatusView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configured = source["configured"];
	        this.user = source["user"];
	        this.bucket = source["bucket"];
	        this.endpoint = source["endpoint"];
	        this.folders = this.convertValues(source["folders"], FolderView);
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

