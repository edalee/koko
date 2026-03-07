export namespace main {
	
	export class GitHubPR {
	    repo: string;
	    number: number;
	    title: string;
	    author: string;
	    reviewDecision: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new GitHubPR(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repo = source["repo"];
	        this.number = source["number"];
	        this.title = source["title"];
	        this.author = source["author"];
	        this.reviewDecision = source["reviewDecision"];
	        this.url = source["url"];
	    }
	}
	export class SessionInfo {
	    id: string;
	    name: string;
	    dir: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.dir = source["dir"];
	    }
	}

}

