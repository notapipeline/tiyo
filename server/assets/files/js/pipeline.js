// Diagram Grid
var appelement = null;
var editor = null;
var editorchanged = null;
// for attaching links to elements
var port = null

/**
 * Utility function for normalizing marker's path data.
 * Translates the center of an arbitrary path at <0 + offset,0>.
 */
function normalizeMarker(d, offset) {
    var path = new g.Path(V.normalizePathData(d));
    var bbox = path.bbox();
    var ty = - bbox.height / 2 - bbox.y;
    var tx = - bbox.width / 2 - bbox.x;
    if (typeof offset === 'number') tx -= offset;
    path.translate(tx, ty);
    return path.serialize();
}

/**
 * Default link set
 */
links['tcp'] = new joint.dia.Link({
    attrs: {
        '.marker-source': {
            d: normalizeMarker('M5.5,15.499,15.8,21.447,15.8,15.846,25.5,21.447,25.5,9.552,15.8,15.152,15.8,9.552z'),
            fill: 'blue',
            stroke: 'blue',
        },
        line: {
            stroke: 'blue',
        },
        '.connection': { stroke: 'blue' },
    },
    attributes: {
        type: "tcp",
        dest: 443,
        source: 0,
        address: "",
    },
});

links['udp'] =  new joint.dia.Link({
    attrs: {
        '.marker-target': {
            d: normalizeMarker('M 0 -9 L -11.3 -2.3 V -9 L -13 -9 V 9 L -11.3 9 V 1.5 L 0 6.8 Z'),
            fill: 'orange',
            stroke: 'orange',
        },
        line: {
            stroke: 'orange',
        },
        '.connection': { stroke: 'orange' },
    },
    attributes: {
        type: "udp",
        dest: 0,
        source: 0,
        address: "",
    },
});

links['socket'] = new joint.dia.Link({
    attrs: {
        '.marker-target': {
            d: normalizeMarker('M -4 -5 L -4 -6 c 2 0 9 0 9 6 c 0 6 -7 6 -9 6 L -4 5 C -3 5 4 5 4 0 L 4 0 C 4 -5 -3 -5 -4 -5'),
            fill: 'red',
            stroke: 'red',
        },
        line: {
            stroke: 'red',
        },
        '.connection': { stroke: 'red' },
    },
    attributes: {
        type: "socket",
        path: "",
    },
});

links['file'] = new joint.dia.Link({
    attrs: {
        '.marker-target': { d: 'M 10 0 L 0 5 L 10 10 z' },
    },
    attributes: {
        type: "file",
        path: "",
        watch: false,
    },
});

// Setup pipeline model
(function() {
    graph = new joint.dia.Graph;
    paper = new joint.dia.Paper({
        el: $('#paper-pipeline'),
        model: graph,
        width: 1080,
        height: 720,
        gridSize: 10,
        drawGrid: true,
        restrictTranslate: true,
        background: {
            color: 'rgba(232, 232, 232, 0.3)'
        },
        defaultLink: links['file'].clone(),
        markAvailable: true,
        validateConnection: function(cellViewS, magnetS, cellViewT, magnetT) {
            if (magnetS && magnetS.getAttribute('port-group') === 'in') return false;
            if (cellViewS === cellViewT) return false;
            return magnetT && magnetT.getAttribute('port-group') === 'in';
        },
    });

    this.$tcpProperties = $(
        '<div class="tcpProperties">'+
        '<h4></h4>'+
        '<form>'+
        '  <table>'+
        '    <tr>'+
        '      <td><label for="tcpsourceport">Source port</label></td>'+
        '      <td><input id="tcpsourceport" value="0" /></td>'+
        '    </tr>'+
        '    <tr>'+
        '      <td><label for="tcpdestport">Destination port</label></td>'+
        '      <td><input id="tcpdestport" value="" /></td>'+
        '    </tr>'+
        '    <tr>'+
        '      <td><label for="tcpaddress">Address</label></td>'+
        '      <td><input id="tcpaddress" value=""></td>'+
        '    </tr>'+
        '  </table>'+
        '  <div style="float: right;">'+
        '    <a class="uk-button-small cancel">cancel</a>'+
        '    <a class="uk-button-small uk-button-primary done">done</a>'+
        '  </div>'+
        '</form>'+
        '</div>'
    );

    this.$socketProperties = $(
        '<div class="socketProperties">'+
        '<h4></h4>'+
        '<form>'+
        '  <table>'+
        '    <tr>'+
        '      <td><label for="socketpath">Path</label></td>'+
        '      <td><input id="socketpath" value="" /></td>'+
        '    </tr>'+
        '    <tr>'+
        '      <td><label for="socketwatch">Watch</label></td>'+
        '      <td><input type="checkbox" id="socketwatch" /></td>'+
        '    </tr>'+
        '  </table>'+
        '  <div style="float: right;">'+
        '    <a class="uk-button-small cancel">cancel</a>'+
        '    <a class="uk-button-small uk-button-primary done">done</a>'+
        '  </div>'+
        '</form>'+
        '</div>'
    );

    this.$appProperties = $(
        '<div class="applicationProperties">'+
        '<h4></h4>'+
        '<form>'+
        '  <table>'+
        '    <tr>'+
        '      <td><label for="appname">name</label></td>'+
        '      <td><input id="appname" value="" /></td>'+
        '    </tr>'+
        '    <tr>'+
        '      <td><label for="appcmd">command</label></td>'+
        '      <td><input id="appcmd" value="" /></td>'+
        '    </tr>'+
        '    <tr>'+
        '      <td><label for="appargs">arguments</label></td>'+
        '      <td><input id="appargs" value=""></td>'+
        '    </tr>'+
        '    <tr>'+
        '      <td><label for="appversion">version</label></td>'+
        '      <td><input id="appversion" value=""></td>'+
        '    </tr>'+
        '    <tr>'+
        '      <td><label for="apptimeout">Timeout</label></td>'+
        '      <td><input id="apptimeout" value=""></td>'+
        '    </tr>'+
        '    <tr>'+
        '      <td><label for="appscript">script</label></td>'+
        '      <td><input type="checkbox" id="appscript" />'+
        '          <input type="button" id="editappscript" value="edit" onclick="showEditor()" />' +
        '          <input type="hidden" id="scriptcontent" value="" /><td>'+
        '    </tr>'+
        '  </table>'+
        '  <div style="float: right;">'+
        '    <a id="appcancel" class="uk-button-small">cancel</a>'+
        '    <a id="appdone" class="uk-button-small uk-button-primary">done</a>'+
        '  </div>'+
        '</form>'+
        '</div>'
    );
    
    this.$containerProperties = $(
        '<div class="containerProperties">'+
        '<h4></h4>'+
        '<form>'+
        '  <table>'+
        '    <tr>'+
        '      <td><label for="containername">name</label></td>'+
        '      <td><input id="containername" value="" /></td>'+
        '    </tr>'+
        '    <tr>'+
        '      <td><label for="containerscale">Scale</label></td>'+
        '      <td><input id="containerscale" value=""></td>'+
        '    </tr>'+
        '  </table>'+
        '  <div style="float: right;">'+
        '    <a class="uk-button-small cancel">cancel</a>'+
        '    <a class="uk-button-small uk-button-primary done">done</a>'+
        '  </div>'+
        '</form>'+
        '</div>'
    );
    
    

    $('#paper-pipeline').append(this.$tcpProperties);
    $('#paper-pipeline').append(this.$socketProperties);
    $('#paper-pipeline').append(this.$appProperties);
    $('#paper-pipeline').append(this.$containerProperties);

    /**
     * Base model for droppable items
     */
    joint.shapes.container = {};
    joint.shapes.container.Element = joint.shapes.devs.Model.extend({
        defaults: joint.util.deepSupplement({
            markup: '<g class="rotatable"><g class="scalable"><image class="body"/></g><text class="label"/><g class="inPorts"/><g class="outPorts"/></g>',
            type: 'container.Element',
            perpendicularLinks: true,

            name: "",
            lang: "",
            command: "",
            arguments: "",
            script: false,
            scriptcontent: "",
            custom: false,
            timeout: 15,
            existing: false,

            position: { x: 50, y: 50 },
            size: { width: 50, height: 50 },
            inPorts: ['a', 'b', 'c'],
            outPorts: ['o'],
            attrs: {
                '.label': {
                    text: 'Model',
                    'ref-x': 0.5,
                    'ref-y': 50,
                    'font-size': '10pt',
                },
            },
            ports: {
                groups: {
                    'in': {
                        attrs: {
                            '.port-body': {
                                magnet: 'passive',
                                r: 3,
                                stroke: 'green',
                                fill: 'green',
                                'stroke-width': 1,
                                'ref-x': -8,
                            },
                            '.port-label': {
                                'fill-opacity': 0,
                            },
                        }
                    },
                    'out': {
                        attrs: {
                            '.port-body': {
                                r: 3,
                                stroke: 'red',
                                fill: 'red',
                                'stroke-width': 1,
                                'ref-x': 8,
                            },
                            '.port-label': {
                                'fill-opacity': 0,
                            },
                        }
                    }
                }
            },
        }, joint.shapes.devs.Model.prototype.defaults)
    });

    // Sets (daemon-set, stateful-set, deployment, etc)
    joint.shapes.container.Container = joint.shapes.basic.Generic.extend({
        markup: '<g class="rotatable"><g class="scalable"><rect/></g><image/><text/></g>',
        defaults: joint.util.deepSupplement({
            type: 'container.Container',
            name: "",
            scale: 0,
            settype: "",
            size: { width: 250, height: 250 },
            attrs: {
                rect: { fill: '#fff', 'fill-opacity': '0', stroke: 'black', width: 100, height: 60 },
                text: { 'font-size': 14, text: '', 'ref-x': .5, 'ref-y': -15, ref: 'rect', 'y-alignment': 'top', 'x-alignment': 'middle', fill: 'black' },
                image: { 'ref-x': -15, 'ref-y': -15, ref: 'rect', width: 40, height: 40 },
            }
        }, joint.shapes.basic.Generic.prototype.defaults)
    });

    /**
     * For each SVG in assets/files/img/languages create a container object for
     * drag-drop from the sidebar to the main panel
     */
    waitForLanguages(function() {
        for(const lang in languages) {
            $.get('/static/img/languages/' + lang + '.svg', function(data) {
                languages[lang] = new joint.shapes.container.Element({
                    script: true,
                    custom: true,
                    lang: lang,
                    attrs: {
                        '.body': {
                            'xlink:href': 'data:image/svg+xml;utf8,' + encodeURIComponent(new XMLSerializer().serializeToString(data.documentElement))
                        },
                    },
                });
            });
        }
    });

    /**
     * For each SVG in assets/files/img/languages create a container object for
     * drag-drop from the sidebar to the main panel
     */
    waitForKubernetes(function() {
        for(const kube in kubernetes) {
            $.get('/static/img/kubernetes/' + kube+ '.svg', function(data) {
                kubernetes[kube] = new joint.shapes.container.Container({
                    scale: 0,
                    attrs: {
                        '.body': {
                            'xlink:href': 'data:image/svg+xml;utf8,' + encodeURIComponent(new XMLSerializer().serializeToString(data.documentElement))
                        },
                    },
                });
            });
        }
    });

    /**
     * MOUSE EVENTS
     */
    var attach = false;
    var attached = false;
    var dragging = false;
    var dragStartPosition = null;
    var mousedown = false;

    paper.on('blank:mousewheel', (event, x, y, delta) => {
        const scale = paper.scale();
        // too small to render below 0.25
        if (scale.sx > 0.25 || delta > 0) {
            paper.scale(scale.sx + (delta * 0.25), scale.sy + (delta * 0.25));
        }
    });

    paper.on('blank:pointerdown', (event, x, y, delta) => {
        closeAll();
        dragging = true;
        dragStartPosition = { x: x, y: y};
    });
    
    paper.on('blank:pointerup', function(cellView, x, y) {
        dragging = false;
        console.log(x, y);
    });

    $('#paper-pipeline').mousemove(function(event) {
        if (dragging) {
            paper.translate(
                event.offsetX - dragStartPosition.x,
                event.offsetY - dragStartPosition.y
            );
        }
    });
    
    paper.on({
        'element:contextmenu': onElementRightClick,
        'link:contextmenu': onLinkRightClick,
    });
    
    paper.on('element:mouseover', function(view, evt) {
        event = evt;
        var port = view.findAttribute('port', evt.target);
        if (activeLink && activeLink.attributes.type == 'link' && port && port == 'o') {
            // hover for half a second before attaching
            attach = true;
            setTimeout(function() {
                if (attach) {
                    activeLink.source({id: view.model.id, port: port }).addTo(graph);
                    activeLink.target(getTransformPoint());
                    attached = true;
                }
            }.bind(this), 500);
        }
    });
    
    /*paper.on('element:mouseleave', (elementView) => {
        elementView.removeTools();
    });
    
    paper.on('element:mouseout', function(view, evt) {
    */
    paper.on('element:mouseleave', function(view, evt) {
        setTimeout(function(v) { v.removeTools() }, 500, view);
        attach = false;
        if (!activeLink || !attached) { return; }
        linkView = activeLink.findView(this);
        if (!linkView) {
            return;
        }
        linkView.startArrowheadMove('target');
        
        $(document).on({
            'mousemove.link': onDrag,
            'mouseup.link': onDragEnd
        }, {
            view: linkView,
            paper: this
        });

        function onDrag(evt) {
            // transform client to paper coordinates
            var p = evt.data.paper.snapToGrid({
                x: evt.clientX,
                y: evt.clientY
            });
            evt.data.view.pointermove(evt, p.x, p.y);
        }

        function onDragEnd(evt) {
            evt.data.view.pointerup(evt);
            $(document).off('.link');
            activeDragEvent = null;
            attached = false;
            activeLink = null;
            /*
             * Prior events dont seem to have propagated
             * by the time this point is reached, therefore
             * setting pointer-events doesn't have an affect.
             * 
             * To bypass this, sleep for 50ms before clearing
             * the pointer events back from "none" to "all".
             */
            setTimeout(function() {
                paper.findViewByModel(
                    graph.getLastCell()
                ).model.attr(
                    './style', 'pointer-events: all'
                );
            }, 50);
        }
    });

    paper.on('element:mouseenter', (elementView) => {
        if (elementView.model.attributes.type == 'container.Element') {
            elementView.addTools(
                new joint.dia.ToolsView({
                    tools: [
                        new joint.elementTools.Remove({
                            useModelGeometry: true, y: '0%', x: '100%',
                        }),
                    ],
                })
            );
        } else {
            elementView.addTools(
                new joint.dia.ToolsView({
                    tools: [
                        new joint.elementTools.Remove({
                            useModelGeometry: true, y: '0%', x: '100%',
                        }),
                        new joint.elementTools.Button({
                            useModelGeometry: true, y: '100%', x: '100%',
                            markup: [{
                                tagName: 'circle',
                                attributes: {
                                    r: 7,
                                    fill: '#000',
                                    'cursor': 'pointer',
                                }
                            }],
                            action: function(evt) {
                                $(document).on({
                                    'mousemove.container': onDrag,
                                    'mouseup.container': onDragEnd
                                }, {
                                    view: elementView
                                });

                                function onDrag(evt) {
                                    // transform client to paper coordinates
                                    var p = paper.snapToGrid({
                                        x: evt.clientX,
                                        y: evt.clientY
                                    });
                                    var attributes = evt.data.view.model.attributes;
                                    x = (attributes.position.x + p.x);
                                    y = (attributes.position.y + p.y);
                                    evt.data.view.model.resize( x, y);
                                    
                                }

                                function onDragEnd(evt) {
                                    $(document).off('mousemove.container');
                                    $(document).off('mouseup.container');
                                }
                            }
                        }),
                    ],
                })
            );
        }
    });
    
    paper.on('element:pointerup', function(view, evt) {
        var port = view.findAttribute('port', evt.target);
        if (elem && elem.attributes.type == 'link' && port && port != 'o') {
            elem.target({id: view.model.id });
        }
    });
    
    
    paper.on('cell:pointerdown', function(cellView, evt, x, y) {
        var cell = cellView.model;
        if (!cell.get('embeds') || cell.get('embeds').length === 0) {
            // Show the dragged element above all the other cells (except when the
            // element is a parent).
            cell.toFront();
            graph.getConnectedLinks(cell).forEach(function(link){link.toFront()});
        }
        if (cell.get('parent')) {
            graph.getCell(cell.get('parent')).unembed(cell);
        }
    });

    paper.on('cell:pointerup', function(cellView, evt, x, y) {
        var cell = cellView.model;
        var cellViewsBelow = paper.findViewsFromPoint(cell.getBBox().center());

        if (cellViewsBelow.length) {
            
            var cellViewBelow = _.find(cellViewsBelow, function(c) { return c.model.id !== cell.id });
            // Prevent recursive embedding.
            if (cellViewBelow && cellViewBelow.model.get('parent') !== cell.id) {
                // only allow embedding to container types but don't allow container nesting (yet)
                if (cellViewBelow.model.attributes.type == 'container.Container' && cell.attributes.type != 'container.Container') {
                    cellViewBelow.model.embed(cell);
                }
            }
        }
    });
    
    /**
     * Close all property dialogues
     */
    function closeAll() {
        ['.applicationProperties', '.tcpProperties',
         '.socketProperties', '.containerProperties',
        ].forEach(function(el){
            $(el).css({
                'display': 'none',
            });
        });
    }

    /**
     * Show properties on right click against a link
     */
    function onLinkRightClick(model, evt, x, y) {
        closeAll();
        var attributes = model.model.attributes.attributes;
        var element = null;
        if (['tcp', 'udp'].includes(attributes.type)) {
            element = $('.tcpProperties');
            $('#tcpsourceport').val(attributes.source);
            $('#tcpdestport').val(attributes.dest);
            $('#tcpaddress').val(attributes.address);
        } else if (['socket', 'file'].includes(attributes.type)) {
            element = $('.socketProperties');
            $('#socketpath').val(attributes.path);
            $('#socketwatch').prop('disabled', true);
            $('#socketwatch').prop('checked', false);
            if (attributes.type == 'file') {
                $('#socketwatch').prop('disabled', false);
                $('#socketwatch').prop('checked', attributes.watch);
            }
            
        }
        element.find('h4').text(attributes.type + ' properties');

        element.css({
            "position": "absolute",
            "display": "block",
            "left": evt.offsetX,
            "top": evt.offsetY,
        });
        
        element.find('.done').click(function(e) {
            if (['tcp', 'udp'].includes(attributes.type)) {
                attributes.source = $('#tcpsourceport').val();
                attributes.dest = $('#tcpdestport').val();
                attributes.address = $('#tcpaddress').val();
            } else if (['socket', 'file'].includes(attributes.type)) {
                attributes.path = $('#socketpath').val();
                attributes.watch = $('#socketwatch').prop('checked');
            }
            element.css({
                "display": "none",
            });
            savePipeline();
            element.find('.done').off('click');
        });
        
        element.find('.cancel').click(function(e) {
            element.css({
                "display": "none",
            });
            element.find('.cancel').off('click');
        });
    }

    /**
     * Show properties on right click against an element
     */
    function onElementRightClick(model, evt, x, y) {
        closeAll();
        if (model.model.attributes.type == 'container.Container') {
            containerAttributes(model, evt, x, y);
        } else {
            appAttributes(model, evt, x, y);
        }
    }
    
    function containerAttributes(model, evt, x, y) {
        var element = $('.containerProperties');
        appelement = model.model;
        $('#containername').val(appelement.attributes.name);
        $('#containerscale').val(appelement.attributes.scale);
        
        element.css({
            "position": "absolute",
            "display": "block",
            "left": evt.offsetX,
            "top": evt.offsetY,
        });

        element.find('.done').click(function(e) {
            appelement.attributes.name = $('#containername').val();
            appelement.attributes.scale = parseInt($('#containerscale').val(), 10);
            appelement.attr()['text'].text = $('#containername').val();
            
            element.css({
                "display": "none",
            });
            savePipeline();
            element.find('.done').off('click');
            joint.dia.ElementView.prototype.render.apply(model, arguments);
        });
        
        element.find('.cancel').click(function(e) {
            element.css({
                "display": "none",
            });
            element.find('.cancel').off('click');
        });
    }
    
    function appAttributes(model, evt, x, y) {
        var element = $('.applicationProperties');
        element.find('h4').text('application properties');
        appelement = model.model;

        $('#appname').prop('disabled', false);
        $('#appname').val(appelement.attr()['.label'].text);
        if (appelement.attr()['.label'].text) {
            $('#appname').prop('disabled', true);
        }

        if (appelement.attributes.command == "") {
            appelement.attributes.command = appelement.attr()['.label'].text
        }

        $('#appcmd').val(appelement.attributes.command);
        $('#appargs').val(appelement.attributes.arguments);
        $('#appversion').val(appelement.attributes.version);
        $('#aptimeout').val(appelement.attributes.timeout);

        if (appelement.attributes.script) {
            $('#appscript').prop('disabled', false);
            $('#appscript').prop('checked', true);
            $('#editappscript').prop('disabled', false);
        } else {
            $('#appscript').prop('disabled', true);
            $('#appscript').prop('checked', false);
            $('#editappscript').prop('disabled', true);
        }

        element.css({
            "position": "absolute",
            "display": "block",
            "left": evt.offsetX,
            "top": evt.offsetY,
        });

        $('#appdone').click(function(e) {
            if (!appelement) {
                return;
            }
            appelement.attr()['.label'].text = $('#appname').val();
            appelement.attributes.name = $('#appname').val();
            appelement.attributes.command = $('#appcmd').val();
            appelement.attributes.arguments = $('#appargs').val();
            appelement.attributes.version = $('#appversion').val();
            appelement.attributes.timeout = parseInt($('#apptimeout').val(), 10);
            appelement.attributes.script = $('#appscript').prop('checked');
            joint.dia.ElementView.prototype.render.apply(model, arguments);
            element.css({
                "display": "none",
            });
            savePipeline();
            appelement = null;
            $('#appdone').off('click');
        });
        
        $('#appcancel').click(function(e){
            element.css({
                "display": "none",
            });
            appelement = null;
            $('#appcancel').off('click');
        });
    }
})();

/**
 * Show the editor dialog
 */
function showEditor()
{
    editor = ace.edit("editor");
    editor.setTheme("ace/theme/xcode");
    if (appelement.attributes.scriptcontent !== "") {
        editor.session.setValue(atob(appelement.attributes.scriptcontent));
        editor.session.$modified = false;
    }

    editor.session.on('change', function(delta) {
        editorchanged = delta;
    });
    editor.session.setMode('ace/mode/' + appelement.attributes.lang);

    UIkit.modal('#scriptentry', {
        escClose: false,
        bgClose: false,
        stack: true,
    }).show();
}

/**
 * Cancel script editing
 */
function cancelScript()
{
    if (editorchanged) {
        UIkit.modal.confirm('You have unsaved changes. Are you sure you wish to close the editor?', {stack: true}).then(function() {
            destroyEditor();
        }, function () {
            showEditor();
        }).catch(e => {
            console.log(e);
        });
    }
}

/**
 * Save script as base64 encoded data into the current element model
 */
function saveScript()
{
    appelement.attributes.scriptcontent = btoa(editor.getValue());
    destroyEditor();
}

/**
 * Destroy the editor after saving/cancelling so its clear for the next element
 */
function destroyEditor()
{
    editor.destroy();
    editor = null;
    editorchanged = null;
    UIkit.modal('#scriptentry').hide();
}
