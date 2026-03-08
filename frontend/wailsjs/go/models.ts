export namespace main {
	
	export class AppConfig {
	    slackToken: string;
	    githubRepos: string[];
	    slackEnabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slackToken = source["slackToken"];
	        this.githubRepos = source["githubRepos"];
	        this.slackEnabled = source["slackEnabled"];
	    }
	}
	export class FileChange {
	    path: string;
	    status: string;
	    staged: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.status = source["status"];
	        this.staged = source["staged"];
	    }
	}
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
	export class SlackMessage {
	    type: string;
	    channel: string;
	    channelId: string;
	    user: string;
	    text: string;
	    timestamp: string;
	    teamId: string;
	    unread: boolean;
	    time: number;
	
	    static createFrom(source: any = {}) {
	        return new SlackMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.channel = source["channel"];
	        this.channelId = source["channelId"];
	        this.user = source["user"];
	        this.text = source["text"];
	        this.timestamp = source["timestamp"];
	        this.teamId = source["teamId"];
	        this.unread = source["unread"];
	        this.time = source["time"];
	    }
	}

}

