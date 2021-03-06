<!DOCTYPE html>
<html>
    <head>
        <title>[[ .Title ]]</title>
        <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/uikit@3.6.11/dist/css/uikit.min.css" />
        <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/jointjs@3.2.0/dist/joint.min.css" />
        <link rel="stylesheet" type="text/css" href="/static/css/dashboard.css">

        <script src="https://cdn.jsdelivr.net/npm/js-cookie@rc/dist/js.cookie.min.js"></script>
        <script src="https://cdn.jsdelivr.net/npm/jquery@3.5.1/dist/jquery.min.js"></script>
        <script src="https://cdn.jsdelivr.net/npm/uikit@3.6.11/dist/js/uikit.min.js"></script>
        <script src="https://cdn.jsdelivr.net/npm/uikit@3.6.11/dist/js/uikit-icons.min.js"></script>
        <script src="https://cdn.jsdelivr.net/npm/navigo@8.6.3/lib/navigo.min.js"></script>

        <!-- Joint JS and requirements -->
        <script src="https://cdn.jsdelivr.net/npm/lodash@4.17.20/lodash.min.js"></script>
        <script src="https://cdn.jsdelivr.net/npm/backbone@1.4.0/backbone.min.js"></script>
        <script src="https://cdn.jsdelivr.net/npm/jointjs@3.2.0/dist/joint.min.js"></script>

        <!-- Gauge chart -->
        <script src="https://unpkg.com/gauge-chart@latest/dist/bundle.js"></script>

        <script type="text/javascript" src="https://cdn.jsdelivr.net/npm/handlebars@4.7.6/dist/handlebars.js"></script>
        <script type="text/javascript" src="/static/js/lib/jquery.editable.min.js"></script>
    </head>
    <body>
        <header id="top-head" class="uk-position-fixed">
            <div class="uk-container uk-container-expand uk-background-primary">
                <nav class="uk-navbar uk-light" data-uk-navbar="mode:click; duration: 250">
                    <div class="uk-navbar-left">
                        <div class="uk-navbar-item uk-hidden@m">
                            <a class="uk-logo" href="#"><img class="custom-logo" src="/static/img/dashboard-logo-white.svg" alt=""></a>
                        </div>
                        <ul class="uk-navbar-nav uk-visible@m">
                            <li>
                                <a href="#">File<span data-uk-icon="icon: triangle-down"></span></a>
                                <div class="uk-navbar-dropdown">
                                    <ul class="uk-nav uk-navbar-dropdown-nav file">
                                        <li><a><span data-uk-icon="icon: file"></span> New</a></li>
                                        <li><a><span data-uk-icon="icon: file-edit"></span> Open</a></li>
                                    </ul>
                                </div>
                            </li>
                            <li>
                                <a href="#">Edit<span data-uk-icon="icon: triangle-down"></span></a>
                                <div class="uk-navbar-dropdown">
                                    <ul class="uk-nav uk-navbar-dropdown-nav edit">
                                        <li><a><span data-uk-icon="icon: file"></span> Environment</a></li>
                                        <li><a><span data-uk-icon="icon: file-edit"></span> Credentials</a></li>
                                    </ul>
                                </div>
                            </li>

                        </ul>
                    </div>
                    <div class="uk-navbar-right">
                        <ul class="uk-navbar-nav">
                            <li>
                                <a class="uk-width-1-1 uk-button uk-button uk-button-small" onclick="pipeline.execute()">Execute</a>
                            </li>
                            <li>
                                <a class="uk-width-1-1 uk-button uk-button uk-button-small" style="color: red;" onclick="pipeline.destroy()" uk-icon="trash">&nbsp;</a>
                            </li>                            <li>
                                <a id="play" class="uk-width-1-1 uk-button uk-button uk-button-small" style="color: red;" onclick="pipeline.playpause()" uk-icon="ban">&nbsp;</a>
                            </li>
                            <li style="margin-left: 50px;">
                                <a class="uk-navbar-toggle" data-uk-toggle data-uk-navbar-toggle-icon href="#offcanvas-nav" title="Offcanvas" data-uk-tooltip></a>
                            </li>
                        </ul>
                    </div>
                </nav>
            </div>
        </header>
        <aside id="left-col" class="uk-light uk-visible@m">
            <div class="left-logo uk-flex uk-flex-middle uk-margin-bottom">
                <img class="custom-logo" src="/static/img/dashboard-logo.svg" alt="">
            </div>
            <nav class="left-nav-wrap">
                <ul class="uk-nav uk-nav-default uk-nav-parent-icon">
                    <li>
                        <a href="/">Dashboard</a>
                    </li>
                    <li>
                        <a href="/pipeline">Pipeline</a>
                    </li>
                    <li>
                        <a href="/buckets">Buckets</a>
                    </li>
                </ul>
            </nav>
            <ul uk-accordion="multiple: true; collapsible: false;">
                <li>
                    <a class="uk-accordion-title">Source</a>
                    <div class="uk-accordion-content uk-flex uk-flex-middle uk-margin-bottom">
                        <ul uk-sortable="handle: .source-element" class="uk-grid-stack uk-height-max-large element-list" id="source-element-list"></ul>
                    </div>
                </li>
                <li>
                    <a class="uk-accordion-title">Kubernetes</a>
                    <div class="uk-accordion-content uk-flex uk-flex-middle uk-margin-bottom">
                        <ul uk-sortable="handle: .kubernetes-element" class="uk-grid-stack uk-height-max-large element-list" id="kubernetes-element-list"></ul>
                    </div>
                </li>
                <li>
                    <a class="uk-accordion-title">Element types</a>
                    <div class="uk-accordion-content uk-flex uk-flex-middle uk-margin-bottom">
                        <ul uk-sortable="handle: .container-element" class="uk-grid-stack uk-height-max-large element-list" id="container-element-list"></ul>
                    </div>
                </li>
                <li>
                    <a class="uk-accordion-title">Link types</a>
                    <div class="uk-accordion-content uk-flex uk-flex-middle uk-margin-bottom">
                        <ul uk-sortable="handle: .pipeline-link" class="uk-grid-stack uk-height-max-large element-list" id="pipeline-link-list">
                            <li class="uk-card uk-card-default uk-card-body element-list-element pipeline-list pipeline-link" uk-tooltip="file connector">
                                <p>FILE</p>
                            </li>
                            <li class="uk-card uk-card-default uk-card-body element-list-element pipeline-link" style="color: blue;" uk-tooltip="tcp connector">
                                <p>TCP</p>
                            </li>
                            <li class="uk-card uk-card-default uk-card-body element-list-element pipeline-link" style="color: orange;" uk-tooltip="udp connector">
                                <p>UDP</p>
                            </li>
                            <li class="uk-card uk-card-default uk-card-body element-list-element pipeline-link" style="color: red;" uk-tooltip="unix socket connector">
                                <p>SOCKET</p>
                            </li>
                        </ul>
                    </div>
                </li>
            </ul>
        </aside>
        <article id="content" class="uk-container-center uk-margin-large-bottom" data-uk-height-viewport="expand: true">

        <!-- Dashboard -->
        <section class="uk-position-top-left" id="dashboard">
            <div class="uk-vertical-align uk-text-center uk-height-1-1"></div>
        </section>

        <!-- Pipeline -->
        <section class="uk-position-medium uk-position-top-left uk-width-1-1" id="pipeline">
            <div id="paper-pipeline-holder" class="uk-grid uk-grid-medium uk-sortable" uk-grid>
                <div class="uk-width-3-4 uk-first-column uk-grid-margin uk-height-1-1 casing">
                    <header>
                        <h3 class="editable pagetitle pipelinetitle">Untitled</h3>
                    </header>
                    <div class="gauges">
                        <fieldset>
                            <legend>Available</legend>
                            <div id="availableCpu"></div>
                            <div id="availableMemory"></div>
                        </fieldset>
                        <fieldset>
                            <legend>Requested</legend>
                            <div id="pipelineCpu"></div>
                            <div id="pipelineMemory"></div>
                        </fieldset>
                    </div>
                    <div id="paper-pipeline"></div>
                </div>
                <aside class="uk-height-1-1 uk-width-1-4">
                    <div class="uk-panel uk-panel-box" data-uk-sticky="{top:35}">
                        <h4 class="uk-nav-header">Applications</h4>
                        <div class="uk-margin">
                            <form class="uk-search uk-search-default">
                                <span class="uk-search-icon-flip" uk-search-icon></span>
                                <input id="pipeline-filter" class="uk-search-input" onkeyup="filterApplications()" type="search" placeholder="filter...">
                            </form>
                        </div>
                        <div id="pipeline-applications">
                            <image src="/static/img/pulse.svg" style="height: 30px;width: 100%;" />
                        </div>
                    </div>
                </aside>
            </div>
        </section>

        <!-- Bucket List -->
        <section class="uk-position-medium uk-position-top-left" id="buckets">
            <h3 class="pagetitle">Buckets</h3>
            <div class="uk-vertical-align uk-text-center uk-height-1-1">
                <div class="uk-vertical-align-middle" style="width: 250px;text-align:left" id="data"></div>
            </div>
        </section>

        <section class="uk-position-medium uk-position-top-left uk-width-1-2" id="scan">
            <h3 class="pagetitle">Contents</h3>
            <div class="uk-vertical-align uk-text-center uk-width-1-1">
                <div class="uk-vertical-align-middle uk-grid uk-width-1-1" id="d">
                    <div class="uk-width-1-3"><input class="uk-form-small" type="text" id="pbucket" placeholder="Bucket name"></div>
                    <div class="uk-width-1-3"><input class="uk-form-small" type="text" id="pkey" placeholder="Key Prefix"></div>
                    <div class="uk-width-1-3"><a class="uk-width-1-1 uk-button uk-button-primary uk-button-small" onclick="scan()">List</a></div>
                </div>
                <div class="uk-vertical-align-middle uk-margin-top" id="pfs"></div>
            </div>
        </section>
        </article>

        <!-- offcanvas -->
        <div id="offcanvas-nav" data-uk-offcanvas="flip: true; overlay: true">
            <div class="uk-offcanvas-bar uk-offcanvas-bar-animation uk-offcanvas-slide">
                <button class="uk-offcanvas-close uk-close uk-icon" type="button" data-uk-close></button>
                <ul class="uk-nav uk-nav-default">
                    <li class="uk-active"><a href="#">Active</a></li>
                    <li class="uk-parent">
                        <a href="#">Parent</a>
                        <ul class="uk-nav-sub">
                            <li><a href="#">Sub item</a></li>
                            <li><a href="#">Sub item</a></li>
                        </ul>
                    </li>
                    <li class="uk-nav-header">Header</li>
                    <li><a href="#js-options"><span class="uk-margin-small-right uk-icon" data-uk-icon="icon: table"></span> Item</a></li>
                    <li><a href="#"><span class="uk-margin-small-right uk-icon" data-uk-icon="icon: thumbnails"></span> Item</a></li>
                    <li class="uk-nav-divider"></li>
                    <li><a href="#"><span class="uk-margin-small-right uk-icon" data-uk-icon="icon: trash"></span> Item</a></li>
                </ul>
                <h3>Title</h3>
                <p>Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
                Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.</p>
            </div>
        </div>

        <!-- Components -->
        <div id="create-bucket" class="uk-flex-top" href="#modal-center" uk-modal>
            <div class="uk-modal-dialog uk-modal-body uk-margin-auto-vertical">
                <h3>Create bucket</h3>
                <form class="uk-panel uk-panel-box uk-form">
                    <input class="uk-input" type="text" id="bucket" placeholder="Bucket name">
                    <div style="float: right; margin-top: 10px;">
                        <button class="uk-button uk-button-small uk-modal-close">Cancel </button>
                        <button class="uk-button uk-button-primary uk-button-small" onclick="createBucket()">Create</button>
                    </div>
                </form>
            </div>
        </div>

        <div id="open-pipeline" class="uk-flex-top" href="#modal-center" uk-modal>
            <div class="uk-modal-dialog uk-modal-body uk-margin-auto-vertical">
                <h3>Open pipeline</h3>
                <div class="content"></div>
                <div style="float: right; margin-top: 10px;">
                    <button class="uk-button uk-button-small uk-modal-close">Cancel </button>
                </div>
            </div>
        </div>

        <div id="environment" class="uk-flex-top" href="#modal-center" uk-modal>
            <div class="uk-modal-dialog uk-modal-body uk-margin-auto-vertical">
                <h3>Environment</h3>
                <div id="environment-content"></div>
                <div id="scriptentrybuttons" style="float: right; margin-top: 10px;">
                    <button class="uk-button uk-button-small uk-modal-close" onclick="pipeline.cancelEnvironment()">Cancel </button>
                    <button class="uk-button uk-button-primary uk-button-small" onclick="pipeline.saveEnvironment()">Save</button>
                </div>
            </div>
        </div>

        <div id="credentials"></div>
        <div id="scriptentry" class="uk-flex-top" href="#modal-center" uk-modal>
            <div class="uk-modal-dialog uk-modal-body uk-margin-auto-vertical">
                <ul uk-tab id="codesource" data-uk-tab="{connect:'#scriptsource'}">
                    <li id="codesource1" class="uk-active"><a href="#">Inline</a></li>
                    <li id="codesource2"><a href="#">Git Repository</a></li>
                    <li id="codesource3"><a href="#">Upload</a></li>
                </ul>
                <ul id="scriptsource" class="uk-switcher uk-margin">
                    <li id="editor" class="uk-active"></li>
                    <li id="gitdetails">
                        <form>
                            <table>
                                <tbody>
                                    <tr>
                                        <td>Repository address</td>
                                        <td><input id="gitrepo" value="" /></td>
                                    </tr>
                                    <tr>
                                        <td>Branch</td>
                                        <td><input id="gitbranch" value="" /></td>
                                    </tr>
                                    <tr>
                                        <td>Username</td>
                                        <td><input id="gituser" value="" /></td>
                                    </tr>
                                    <tr>
                                        <td>Password</td>
                                        <td><input id="gitpass" value="" type="password" /></td>
                                    </tr>
                                    <tr>
                                        <td>Entry-point</td>
                                        <td><input id="gitentry" value="" /></td>
                                    </tr>
                                </tbody>
                            </table>
                        </form>
                    </li>
                    <li><div id="upload">Not implemented</div></li>
                </ul>
                <div id="scriptentrybuttons" style="float: right; margin-top: 10px;">
                    <button class="uk-button uk-button-small uk-modal-close" onclick="pipeline.cancelScript()">Cancel </button>
                    <button class="uk-button uk-button-primary uk-button-small" onclick="pipeline.saveScript()">Save</button>
                </div>
            </div>
        </div>
        <div id="message"><p></p></div>
    </body>


<!-- Mustache templates -->

<script id="sourcetpl" type="x-tmpl-mustache">
{{#list}}
<li class="uk-card uk-card-default uk-card-body source-element element-list-element">
    <image src="/static/img/source/{{.}}.svg" alt="{{.}}" uk-tooltip="{{.}}" />
</li>
{{/list}}
</script>

<script id="kubernetestpl" type="x-tmpl-mustache">
{{#list}}
<li class="uk-card uk-card-default uk-card-body kubernetes-element element-list-element">
    <image src="/static/img/kubernetes/{{.}}.svg" alt="{{.}}" uk-tooltip="{{.}}" />
</li>
{{/list}}
</script>

<script id="containertpl" type="x-tmpl-mustache">
{{#list}}
<li class="uk-card uk-card-default uk-card-body container-element element-list-element">
    <image src="/static/img/container/{{.}}.svg" alt="{{.}}" uk-tooltip="{{.}}" />
</li>
{{/list}}
</script>




<!-- buckets -->
<script id="template" type="x-tmpl-mustache">
    <table class="uk-table">
    <tbody>
    {{#list}}
        <tr><td><a onclick="doPrefixScan('{{.}}')">{{.}}</a></td></tr>
    {{/list}}
    </tbody>
</table>
</script>

<script id="openpipelinetpl" type="x-tmpl-mustache">
<ul>
    {{#each list}}
    <li><a onclick="openPipeline('{{@key}}')">{{@key}}</a></li>
    {{/each}}
</ul>
</script>

<!-- applications -->
<script id="applicationstpl" type="x-tmpl-mustache">
<ul uk-sortable="handle: .uk-sortable-handle" class="uk-grid-stack uk-height-max-large" id="pipeline-apps-list">
    {{#list}}
    <li class="uk-first-column" style="margin-bottom: 5px;">
        <div class="uk-card uk-card-default uk-card-body">
        <span class="uk-sortable-handle uk-margin-small-right uk-text-center uk-icon" uk-icon="icon: table"></span>{{.}}
        </div>
    </li>
    {{/list}}
</ul>
</script>

<!-- Key list -->
<script id="exploretpl" type="x-tmpl-mustache">
{{#if buckets.length}}
  <h3 style="margin-top: 30px;">Buckets<span style="float:right">{{bucketlen}} items</span></h3>
  <ul>
    {{#buckets}}
    <li style="text-align: left;"><a onclick="scan('{{.}}')">{{.}}</a></li>
    {{/buckets}}
  </ul>
{{/if}}
{{#if keylen}}
<h3>Keys<span style="float:right">{{keylen}} items</span></h3>
<table class="uk-table" id="{{id}}">
    <thead>
        <tr>
        <th style="text-align: right;">Key</th>
        <th style="text-align: left;">Value</th>
        <th style="text-align: right;">Delete</th>
        </tr>
    </thead>
    <tbody>
    {{#each keys}}
        <tr>
            <td style="text-align: right;">{{@key}}</td>
            <td style="text-align: left;" class="editable">{{this}}</td>
            <td style="text-align: right;"><a onclick="doDelete('{{@key}}')">[x]</a></td>
        </tr>
   {{/each}}
    </tbody>
</table>
{{/if}}
</script>

<!-- Page load scripts -->
<script src="https://cdnjs.cloudflare.com/ajax/libs/ace/1.4.12/ace.min.js"></script>
<script language="javascript">
ace.config.set("packaged", true);
ace.config.set("basePath", "https://cdnjs.cloudflare.com/ajax/libs/ace/1.4.12/");


var links = [];
var defaultAttrs = {
    kubernetes: {
        scale: 0,
    },
    container: {
        script: true,
        custom: true,
    },
    source: {}
}
</script>

<script src="/static/js/collections/collection.js"></script>
<script src="/static/js/pipeline.js"></script>
<script src="/static/js/page.js"></script>
<script src="/static/js/api.js"></script>
<script src="/static/js/router.js"></script>
</html>
