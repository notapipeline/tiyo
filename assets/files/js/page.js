/* Copyright 2021 The Tiyo authors
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// JointJS elements
var graph = null;
joint.shapes.container = {};
joint.forms = {}

// wiring elements
var elem = null;
var target = null;
var router = null;
var dragEvent = null;
var activeLink = null;
var activeDragEvent = null;

// graph element collections
var collections = {
    source: null,
    kubernetes: null,
    container: null,
    link: null,
}

for (var collection in collections) {
    collections[collection] = new Collection(collection, defaultAttrs[collection]);
    collections[collection].load();
}

var pipeline = null;

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

            var point = pipeline.getTransformPoint();
            var cell = collections.container.clone('dockerfile').position(
                point.x, point.y
            ).attr(
                '.label/text', elem.textContent.trim()
            );

            cell.attributes.name = elem.textContent.trim();
            cell.attributes.script = true;
            cell.attributes.custom = false;
            pipeline.checkEmbed(cell, point);

            target = null;
            elem = null;
        });
    });
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
 * Load the menu bar - different for each page
 */
function loadMenu() {
    $('.file').find('li > a').bind("click", function() {
        var text = $(this).text().toLowerCase().trim();
        switch (router.lastResolved()[0].url) {
            case "pipeline":
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
            case "buckets":
                switch (text) {
                    case "new":
                        Cookies.remove('bucket');
                        UIkit.modal('#create-bucket').show();
                        break;
                    case "open":
                        break;
                }
                break;
            case "scan":
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
    $('.edit').find('li > a').bind("click", function() {
        var text = $(this).text().toLowerCase().trim();
        switch (router.lastResolved()[0].url) {
            case "pipeline":
                switch (text) {
                    case "environment":
                        openEnvironmentPopup(pipeline.graph);
                        break;
                    case "credentials":
                        openCredentialsPopup(pipeline.graph);
                        break;
                }
                break;
        }
    });
}

function openEnvironmentPopup(whatfor) {
    pipeline.appelement = whatfor;
    pipeline.showEnvironment();
}

function openCredentialsPopup(whatfor) {
    pipeline.appelement = whatfor;
    pipeline.showCredentials();
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

var statusCheck = null;
function execute() {
    var pipeline = Cookies.get('pipeline');
    if (pipelineExecuting() || !pipeline) {
        return;
    }

}

function pipelineExecuting() {
    if (router.lastResolved()[0].url != "pipeline") {
        clearInterval(statusCheck);
        return;
    }
    var pipeline = Cookies.get('pipeline');
}

function handleError(err) {
    if (err && err.responseJSON) {
        error(err.responseJSON.message);
    }
}

function displayMessage(element) {
    width = $(window).width();
    containerWidth = element.width();
    leftMargin = (width-containerWidth)/2;
    element.css({
        display: 'block',
        opacity: '100%',
        'margin-left': leftMargin,
    });
    element.fadeOut(5000, () => {
        element.find('p').html("");
        element.css({
            display: 'none',
        });
        element.removeClass('error')
    });
}

function success(message) {
    var element = $('#message');
    element.removeClass();
    element.addClass('success');
    element.find('p').html(message);
    displayMessage(element);
}

function warning(message) {
    var element = $('#message');
    element.removeClass();
    element.addClass('warning');
    element.find('p').html(message);
    displayMessage(element);
}

function error(message) {
    var element = $('#message');
    element.removeClass();
    element.addClass('error');
    element.find('p').html(message);
    displayMessage(element);
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
    loadBucketTable();
    editableElements();
    loadMenu();
});
