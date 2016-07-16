var graph = null;
var paper = null;
var elem = null;
var target = null;
var router = null;
var pipelineAutoSave = null;
var dragEvent = null;

// diagram elements
var languages = []
var links = []
var activeLink = null;
var activeDragEvent = null;

/**
 * Gets a list of available 'languages' from the server.
 * Basically returns the SVG list from assets/files/img/languages
 */
$.get('/api/v1/languages', function (data) {
    var langs = data.message
    langs.sort(function (a, b) {
        return a.toLowerCase().localeCompare(b.toLowerCase());
    });
    langs.forEach(function(img) {
        language = img.split('.')[0]
        languages[language] = null;
    });
});

/**
 * Inline editing of certain headings/table cells
 */
function editableElements() {
    $('.editable').each( function() {
        $(this).editable({
            event: 'dblclick',
            touch: true,
            lineBreaks: false,
            toggleFontSize: false,
            closeOnEnter: true,
            emptyMessage: "Double click to edit",
            tinyMCE: false,
            editorStyle : {},
            callback: function(data) {
                if (data.$el[0].offsetParent.id == "pipeline") {
                    createPipeline(data.$el.text());
                } else if (data.$el[0].tagName.toLowerCase() == "td") {
                    var id = $(data.$el[0]).closest('table').attr('id').split("-").slice(1).join('-');
                    var key = $(data.$el[0]).prev().text();
                    var content = $(data.$el[0]).text();
                    put(id, null, key, content);
                }
            }
        });
    });
}

/**
 * Populates the bucket page with a list of available buckets
 */
function loadBucketTable()
{
    var source = $('#template').html();
    var template = Handlebars.compile(source);

    $.get("/api/v1/bucket",{},function(data){
    var html    = template({list: data.message});
    $('#data').html(html);
  });
}


/**
 * Filter the application sidebar
 */
function filterApplications() {
    var filter = $('#pipeline-filter').val().toLowerCase();
    $("#pipeline-apps-list").children().each(function() {
        if ($(this).text().toLowerCase().includes(filter)) {
            $(this).show();
        } else {
            $(this).hide();
        }
    });
}

/**
 * Populates the application sidebar with a list of docker containers
 * from biocontainers - see go code for details.
 */
function loadApplications()
{
    var source = $('#applicationstpl').html();
    var template = Handlebars.compile(source);

    $.get("/api/v1/containers",{},function(data){
        var html    = template({list: data.message});
        $(html).ready(function() {
            $('#pipeline-applications').html(html);
        });
    });
    
    // start handler for dragging application to grid
    waitForEl('#pipeline-apps-list', function() {
        UIkit.util.on('#pipeline-apps-list', 'start', (e) => {
            target = null;
            elem = e.detail[1];
            document.getElementById('paper-pipeline-holder').addEventListener('pointermove', onDragging);
        });

        // stop handler to attach application to grid
        UIkit.util.on('#pipeline-apps-list', 'stop', (e) => {
            if (!target) {
                return;
            }
            document.getElementById('paper-pipeline-holder').addEventListener('pointermove', onDragging);

            if ($(target)[0].nodeName == "svg" && $(target)[0].parentElement.id == "paper-pipeline") {
                var point = getTransformPoint();
                c = languages['dockerfile'].clone();
                c.attributes.name = elem.textContent.trim();
                c.attributes.script = false;
                c.attributes.custom = false;
                
                c.position(point.x, point.y).attr(
                    '.label/text', elem.textContent.trim()
                ).addTo(graph);
            }
            target = null;
            elem = null;
        });
    });
}

/**
 * Creates a point on the SVG to reference when dropping elements
 */
function getTransformPoint()
{
    var svgPoint = paper.svg.createSVGPoint();
    svgPoint.x = dragEvent.offsetX;
    svgPoint.y = dragEvent.offsetY;
    return svgPoint.matrixTransform(paper.viewport.getCTM().inverse());
}

/**
 * Helper function to clear down after dropping element
 */
function clearAllActive()
{
    target = null;
    elem = null;
    activeLink = null;
    activeDragEvent = null;
}
/*
 * -----------------------------------------------------------------------------------------------
 * Tool bar connections
 * -----------------------------------------------------------------------------------------------
 */

UIkit.util.on('#pipeline-element-list', 'start', (e) => {
    elem = e.detail[1];
    document.getElementById('paper-pipeline-holder').addEventListener('pointermove', onDragging);
});


UIkit.util.on('#pipeline-element-list', 'stop', (e) => {
    if (!target) {
        return;
    }
    document.getElementById('paper-pipeline-holder').removeEventListener('pointermove', onDragging);
    if ($(target)[0].nodeName == "svg" && $(target)[0].parentElement.id == "paper-pipeline") {
        var point = getTransformPoint();
        languages[
            $(elem).find('img').attr('src').replace(/.*\//, '').split('.')[0]
        ].clone().position(point.x, point.y).attr(
            '.label/text', elem.textContent.trim()
        ).addTo(graph);
    }
    target = null;
    elem = null;
});

// links
UIkit.util.on('#pipeline-link-list', 'start', (e) => {
    activeDragEvent = e;
    elem = e.detail[1];
    activeLink = links[
        $(elem).find('p').text().toLowerCase()
    ].clone();
    document.getElementById('paper-pipeline-holder').addEventListener('pointermove', onDragging);
});

UIkit.util.on('#pipeline-link-list', 'stop', (e) => {
    if (!target) {
        return;
    }
    document.getElementById('paper-pipeline-holder').removeEventListener('pointermove', onDragging);
    target = null;
    elem = null;
});

/*
 * -----------------------------------------------------------------------------------------------
 * End of toolbar applications
 * -----------------------------------------------------------------------------------------------
 */

/**
 * Sets an interval timer to wait for a html element to become ready
 */
function waitForEl(selector, callback) {
    var poller = setInterval(function() {
        $jObject = jQuery(selector);
        if ($jObject.length < 1) {
            return;
        }
        clearInterval(poller);
        callback($jObject);
    },100);
}

/**
 * Update the current event/target when dragging
 */
function onDragging(e) {
    dragEvent = e;
    target = e.target;
}

/**
 * Saves the pipeline back to the KV store
 */
function savePipeline() {
    if (pipelineAutoSave) {
        clearInterval(pipelineAutoSave);
        pipelineAutoSave = null;
    }

    if (router._lastRouteResolved.url != "/pipeline") {
        return;
    }
    var title = $('.editable.pipelinetitle').text();
    if (title != "Untitled" && graph.toJSON().cells.length > 0) {
        console.log('Saving pipeline ' + title);
        put('pipeline', null, title, btoa(JSON.stringify(graph.toJSON())));
        createFileStore(title);
        Cookies.set('pipeline', title);
    }
    if (!pipelineAutoSave) {
        pipelineAutoSave = setInterval(savePipeline, 60000);
    }
}

/**
 * Load the pipeline from KV store
 */
function loadPipeline() {
    console.log('Valid for pipeline?', router._lastRouteResolved.url);
    if (router._lastRouteResolved.url != "/pipeline") {
        return;
    }
    var pipelineValue = Cookies.get('pipeline');
    console.log('Loading pipeline ' + pipelineValue);
    if (pipelineValue && pipelineValue !== "") {
        $.get('/api/v1/bucket/pipeline/' + encodeURI(pipelineValue), function(data, status) {
            if (data && data.code == 200) {
                graph.fromJSON(JSON.parse(atob(data.message)));
                $('.editable.pipelinetitle').text(Cookies.get('pipeline'));
            }
        }).fail(function(e) {
            Cookies.remove('pipeline');
        });
    }
}

/**
 * Load the menu bar - different for each page
 */
function loadMenu() {
    $('.file').find('li > a').bind("click", function() {
        var text = $(this).text().toLowerCase().trim();
        switch (router._lastRouteResolved.url) {
            case "/pipeline":
                switch (text) {
                    case "new":
                        Cookies.remove('pipeline');
                        window.location.assign('/pipeline');
                        break;
                    case "open":
                        openPipelinePopup();
                        break;
                }
                break;
            case "/buckets":
                switch (text) {
                    case "new":
                        Cookies.remove('bucket');
                        UIkit.modal('#create-bucket').show();
                        break;
                    case "open":
                        break;
                }
                break;
            case "/scan":
                switch (text) {
                    case "new":
                        UIkit.modal('#create-bucket').show();
                        break;
                    case "open":
                        break;
                }
                break;
        }
    });
}

/**
 * Convert the list of languages into droppable items
 */
function loadLanguages()
{
    var source = $('#languagestpl').html();
    var template = Handlebars.compile(source);
    var html = template({id: "languagestpl", list: Object.keys(languages)});
    $('#pipeline-element-list').html(html);
}

/**
 * Fake file handler for opening pipelines from KV store
 */
function openPipelinePopup() {
    $('#open-pipeline > div > div.content').html("");
    var source = $('#openpipelinetpl').html();
    var template = Handlebars.compile(source);

    $.get("/api/v1/scan/pipeline", function(data) {
        var html = template({id: "openpipelinetpl", list: data.message['keys']});
        $('#open-pipeline > div > div.content').html(html);
    });
    UIkit.modal('#open-pipeline').show();
}

/**
 * Fake open event to load a pipeline
 */
function openPipeline(pipelineName)
{
    Cookies.remove('pipeline');
    Cookies.set('pipeline', pipelineName);
    window.location.assign('/pipeline');
}

$(document).ready(function() {
    router.resolve();
    loadLanguages();
    loadBucketTable();
    editableElements();
    loadMenu();
});
