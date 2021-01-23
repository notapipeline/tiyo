// Setup pipeline model
class Pipeline {

    // every 5 seconds
    INTERVAL = 5000;

    attach = false;
    attached = false;
    dragging = false;
    dragStartPosition = null;
    dragEvent = null;
    mousedown = false;

    appelement = null;
    editor = null;
    editorchanged = null;

    // for attaching links to elements
    port = null;
    autoSave = null;
    pipeline = "";
    executing = false;

    constructor() {
        this.graph = new joint.dia.Graph;
        console.log(collections.link.clone('file'));
        this.paper = new joint.dia.Paper({
            el: $('#paper-pipeline'),
            model: this.graph,
            width: 1280,
            height: 720,
            gridSize: 10,
            drawGrid: true,
            restrictTranslate: true,
            background: {
                color: 'rgba(232, 232, 232, 0.3)'
            },
            defaultLink: collections.link.clone('file'),
            markAvailable: true,
            validateConnection: function(viewS, magnetS, viewT, magnetT) {
                if (magnetS && magnetS.getAttribute('port-group') === 'in') return false;
                if (viewS === viewT) return false;
                return magnetT && magnetT.getAttribute('port-group') === 'in';
            },
        });
        $('#play').attr('uk-icon', 'ban');
        $('#play').css({
            color: '#FF0000',
        });
    }

    save() {
        if (this.autoSave) {
            clearInterval(this.autoSave);
            this.autoSave = null;
        }

        if (router.lastResolved()[0].url != "pipeline") {
            return;
        }

        var title = $('.editable.pipelinetitle').text();
        if (title != "Untitled" && this.graph.toJSON().cells.length > 0) {
            console.log('Saving pipeline' + title);
            put('pipeline', null, title, btoa(JSON.stringify(this.graph.toJSON())));
            createFileStore(title);
            Cookies.set('pipeline', title);
            this.pipeline = title;
        }

        if (!this.autoSave) {
            this.autoSave = setInterval(this.save.bind(this), 60000);
        }
    }

    load() {
        console.log('Valid for pipeline?', router.lastResolved()[0].url);
        if (router.lastResolved()[0].url != "pipeline") {
            return;
        }
        this.pipeline = Cookies.get('pipeline');
        console.log('Loading pipeline ' + this.pipeline);
        if (this.pipeline !== "") {
            $.get('/api/v1/bucket/pipeline/' + encodeURI(this.pipeline), function(data, status) {
                if (data && data.code == 200) {
                    this.graph.fromJSON(JSON.parse(atob(data.message)));
                    $('.editable.pipelinetitle').text(Cookies.get('pipeline'));
                    this.status();
                }
            }.bind(this)).fail(function(e) {
                Cookies.remove('pipeline');
            });
        }
    }

    execute() {
        this.executing = true;
        $("#execute").css({
            'pointer-events': 'none'
        });

        $('#destroy').css({
            'pointer-events': 'all'
        });

        $.post("/api/v1/execute", JSON.stringify({
                pipeline: this.pipeline,
            }),
            function (data, status) {
                this.status();
            }.bind(this)
        ).fail(function(e) {
            $('#message').addClass('uk-alert-warning');
            console.log(e);
            $('#message').find('p').html('Failed to create bucket');
        });
    }

    isExecuting() {
        return this.executing;
    }

    status() {
        if (this.statusCheck == null) {
            this.statusCheck = setInterval(this.status.bind(this), this.INTERVAL);
        }

        $.get("/api/v1/status/" + encodeURI(this.pipeline), function(data) {
            $("#execute").prop('disabled', true);
            console.log(data.message);

            var status = data["message"]["status"];
            var groups = data.message.groups;

            var color = '#000';
            for (var key in groups) {
                var group = groups[key];
                var groupContainer = this.graph.getCell(key);
                var groupContainerView = this.paper.findViewByModel(groupContainer);

                console.log(group.state)
                switch (group.state) {
                    case 'Failed':
                        color = '#FF0000';
                        break;
                    case 'Ready':
                    case 'Running':
                        color = '#0000FF';
                        break;
                    case 'Executing':
                        color = '#00FF00';
                        break;
                    case 'Terminated':
                    case 'Terminating':
                        color = '#7A581D';
                        break;
                    case 'Creating':
                    case 'Pending':
                        color = '#D66304';
                        break;
                }

                groupContainerView.model.attributes.attrs.rect.stroke = color;
                joint.dia.ElementView.prototype.render.apply(groupContainerView);
                var cells = [];
                for (var i=0; i < groupContainer.attributes.embeds.length; i++) {
                    if (this.graph.getCell(groupContainer.attributes.embeds[i]).attributes.type == 'container.Container') {
                        cells.push(groupContainer.attributes.embeds[i]);
                    }
                }
                this.podStatus(group, cells, groupContainer.attributes.scale);
            }
        }.bind(this));
    }

    podStatus(group, containerKeys, expected) {
        var containers = {};
        for (var i = 0; i<containerKeys.length; i++) {
            containers[containerKeys[i]] = {
                Waiting:    0,
                Running:    0,
                Terminated: 0,
            }
        };

        for (var podkey in group.pods) {
            var pod = group.pods[podkey];
            for (var container in pod.containers) {
                var id = pod.containers[container].id;
                containers[id][pod.containers[container].state] += 1;
            }
        }

        for (var key in containers) {
            var containerCell = this.graph.getCell(key);
            var cellView = this.paper.findViewByModel(containerCell);
            var length   = (containers[key].Waiting + containers[key].Running + containers[key].Terminated)
            var width    = Math.floor((length / expected) * 100).toString() + '%';
            var color = '#00FF00';

            // If more containers are waiting than running, color blue
            if (containers[key].Waiting > containers[key].Running) {
                color = '#0000FF';
            }

            // If any container is terminated, set color to red
            if (containers[key].Terminated > 0) {
                color = '#FF0000';
            }

            containerCell.attr('.progress', {
                'ref-width': width,
                'fill': color,
            });
            joint.dia.ElementView.prototype.render.apply(cellView);
        }
        console.log(containers);
    }


    playpause() {
        if (this.executing) {
            this.stop();
            $('#play').attr('uk-icon', 'play-circle');
            $('#play').css({
                color: '#00FF00',
            });
            return;
        }
        this.start();
        $('#play').attr('uk-icon', 'ban');
        $('#play').css({
            color: '#FF0000',
        });
    }

    start() {
        this.executing = true;
        $.post("/api/v1/startflow", JSON.stringify({
                pipeline: this.pipeline,
            }),
            function (data, status) {
                if (this.statusCheck == null) {
                    this.statusCheck = setInterval(this.status.bind(this), this.INTERVAL);
                }
            }.bind(this)
        ).fail(function(e) {
            $('#message').addClass('uk-alert-warning');
            console.log(e);
            $('#message').find('p').html('Failed to create bucket');
        });
    }

    stop() {
        this.executing = false;
        $.post("/api/v1/stopflow", JSON.stringify({
                pipeline: this.pipeline,
            }),
            function (data, status) {
                if (this.statusCheck == null) {
                    this.statusCheck = setInterval(this.status.bind(this), this.INTERVAL);
                }
            }.bind(this)
        ).fail(function(e) {
            $('#message').addClass('uk-alert-warning');
            console.log(e);
            $('#message').find('p').html('Failed to create bucket');
        });
    }

    destroy() {
        $('#execute').css({
            'pointer-events': 'all'
        });
        $('#destroy').css({
            'pointer-events': 'none'
        });
        $.post("/api/v1/destroyflow", JSON.stringify({
                pipeline: this.pipeline,
            }),
            function (data, status) {
                if (this.statusCheck == null) {
                    this.statusCheck = setInterval(this.status.bind(this), this.INTERVAL);
                }
            }.bind(this)
        ).fail(function(e) {
            $('#message').addClass('uk-alert-warning');
            console.log(e);
            $('#message').find('p').html('Failed to create bucket');
        });
    }

    setupEvents() {
        $('#paper-pipeline').mousemove(function(event) {
            this.drag(event)
        }.bind(this));

        this.paper.on({
            'blank:mousewheel':    (event, x, y, delta) => { this.blankMouseWheel(event, x, y, delta); },
            'blank:pointerdown':   (event, x, y) => { this.blankPointerDown(event, x, y); },
            'blank:pointerup':     (event, x, y) => { this.blankPointerUp(event, x, y); },

            'link:contextmenu':    (view, event, x, y) => { this.linkRightClick(view, event, x, y); },

            'element:contextmenu': (view, event, x, y) => { this.elementRightClick(view, event, x, y); },
            'element:mouseover':   (view, event, x, y) => { this.elementMouseOver(view, event, x, y); },
            'element:mouseleave':  (view, event, x, y) => { this.elementMouseLeave(view, event, x, y); },
            'element:mouseenter':  (view, event, x, y) => { this.elementMouseEnter(view, event, x, y) },
            'element:pointerup':   (view, event, x, y) => { this.elementPointerUp(view, event, x, y); },

            'cell:pointerdown':    (view, event, x, y) => { this.cellPointerDown(view); },
            'cell:pointerup':      (view, event, x, y) => { this.cellPointerUp(view); },
        });
    }

    drag(event) {
        if (this.dragging) {
            this.paper.translate(
                event.offsetX - this.dragStartPosition.x,
                event.offsetY - this.dragStartPosition.y
            );
        }
    }

    blankMouseWheel(event, x, y, delta) {
        const scale = this.paper.scale();
        var e = event.originalEvent;

        delta = delta * 0.25;
        var offsetX = (e.offsetX || e.clientX - $(this).offset().left);

        var offsetY = (e.offsetY || e.clientY - $(this).offset().top);
        var p = this.offsetToLocalPoint(offsetX, offsetY);
        var newScale = scale.sx + delta;
        console.log(' delta', delta, 'offsetX ', offsetX, 'offsetY', offsetY, 'p', p, 'scale', scale, 'newScale', newScale)
        if (newScale >= 0.5 && newScale <= 2) {
            this.paper.setOrigin(0, 0);
            this.paper.scale(newScale, newScale, p.x, p.y);
        }
    }

    blankPointerDown(event, x, y) {
        this.closeAll();
        this.dragging = true;
        this.dragStartPosition = { x: x, y: y};
    }

    blankPointerUp(event, x, y) {
        this.dragging = false;
    }

    linkRightClick(view, event, x, y) {
        collections['link'].attributes(view, event, x, y);
    }

    elementRightClick(view, event, x, y) {
        this.closeAll();
        this.appelement = view.model;
        var type = view.model.attributes.type.split('.')[1].toLowerCase()
        collections[type].attributes(view, event, x, y)
    }

    elementMouseOver(view, event, x, y) {
        var port = view.findAttribute('port', event.target);
        if (activeLink && activeLink.attributes.type == 'link' && port && port == 'o') {
            // hover for half a second before attaching
            this.attach = true;
            setTimeout(function() {
                if (this.attach) {
                    activeLink.source({id: view.model.id, port: port }).addTo(this.graph);
                    activeLink.target(this.getTransformPoint());
                    this.attached = true;
                }
            }.bind(this), 500);
        }
    }

    elementMouseLeave(view, event, x, y) {
        setTimeout(function(v) { v.removeTools() }, 500, view);
        this.attach = false;
        // global activeLink
        if (!activeLink || !this.attached) { return; }
        this.linkView = activeLink.findView(this.paper);
        if (!this.linkView) {
            return;
        }
        this.linkView.startArrowheadMove('target');

        $(document).on({
            'mousemove.link': this.linkDragStart,
            'mouseup.link': this.linkDragEnd
        }, {
            view: this.linkView,
            paper: this.paper
        });
    }

    elementMouseEnter(view, event, x, y) {
        view.addTools(this.toolsView(view));
    }

    elementPointerUp(view, event, x, y) {
        var port = view.findAttribute('port', event.target);
        if (elem && elem.attributes.type == 'link' && port && port != 'o') {
            elem.target({id: view.model.id });
        }
    }

    cellPointerDown(view, event, x, y) {
        var cell = view.model;
        if (!cell.get('embeds') || cell.get('embeds').length === 0) {
            // Show the dragged element above all the other cells (except when the
            // element is a parent).
            cell.toFront();
            this.graph.getConnectedLinks(cell).forEach(function(link){link.toFront()});
        }
        if (cell.get('parent')) {
            this.graph.getCell(cell.get('parent')).unembed(cell);
        }
    }

    cellPointerUp(view, event, x, y) {
        var cell = view.model;
        this.checkEmbed(cell, cell.getBBox().center());
    }

    toolsView(view) {
        var attributes = view.model.attributes;
        var slice = attributes.type.split('.');

        var type = slice[slice.length-1];
        if (!collections[type.toLowerCase()].isGroupType()) {
            return new joint.dia.ToolsView({
                tools: [
                    new joint.elementTools.Remove({
                        useModelGeometry: true, y: '0%', x: '100%',
                    }),
                ],
            });
        }

        return new joint.dia.ToolsView({
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
                    action: (event) => {
                        $(document).on({
                            'mousemove.container': (event) => {
                                // transform client to paper coordinates
                                var p = this.paper.snapToGrid({
                                    x: event.clientX,
                                    y: event.clientY
                                });
                                var nw = p.x - event.data.attributes.position.x;
                                var nh = p.y - event.data.attributes.position.y;
                                event.data.view.model.resize( nw, nh);
                            },
                            'mouseup.container': (event) => {
                                $(document).off('mousemove.container');
                                $(document).off('mouseup.container');
                            }
                        }, {
                            attributes: attributes,
                            view: view,
                        });
                    }
                }),
            ],
        });
    }

    linkDragStart(event) {
        var p = event.data.paper.snapToGrid({
            x: event.clientX,
            y: event.clientY
        });
        event.data.view.pointermove(event, p.x, p.y);
    }

    linkDragEnd(event) {
        event.data.view.pointerup(event);
        $(document).off('.link');
        this.activeDragEvent = null;
        this.attached = false;
        /*
         * Prior events dont seem to have propagated
         * by the time this point is reached, therefore
         * setting pointer-events doesn't have an affect.
         *
         * To bypass this, sleep for 50ms before clearing
         * the pointer events back from "none" to "all".
         */
        setTimeout(function() {
            if (activeLink != null) {
                activeLink.attr('./style', 'pointer.events: all');
                activeLink = null;
            }
        }, 50);
    }

    /**
     * Close all property dialogues
     */
    closeAll() {
        Object.keys(collections).forEach(function(collection) {
            collections[collection].close();
        });
    }

    checkEmbed(cell, point) {
        cell.addTo(this.graph);
        var viewsBelow = this.paper.findViewsFromPoint(point);
        if (viewsBelow.length) {
            var viewBelow = _.find(viewsBelow, function(c) { return c.model.id !== cell.id });
            if (viewBelow && viewBelow.model.get('parent') !== cell.id) {
                // only allow embedding to container types but don't allow container nesting (yet)
                if (viewBelow.model.attributes.type == 'container.Kubernetes' && cell.attributes.type != 'container.Kubernetes') {
                    viewBelow.model.embed(cell);
                }
            }
        }
    }

    getTransformPoint() {
        // global dragEvent
        return this.offsetToLocalPoint(dragEvent.offsetX, dragEvent.offsetY)
    }

    offsetToLocalPoint(x, y) {
        var svgPoint = this.paper.svg.createSVGPoint();
        svgPoint.x = x;
        svgPoint.y = y;
        return svgPoint.matrixTransform(this.paper.viewport.getCTM().inverse());
    }

    /**
     * Show the editor dialog
     */
    showEditor() {
        this.editor = ace.edit("editor");
        this.editor.setTheme("ace/theme/xcode");
        if (this.appelement.attributes.scriptcontent !== "") {
            this.editor.session.setValue(atob(this.appelement.attributes.scriptcontent));
            this.editor.session.$modified = false;
        }

        this.editor.session.on('change', function(delta) {
            this.editorchanged = delta;
        }.bind(this));
        this.editor.session.setMode('ace/mode/' + this.appelement.attributes.element);

        UIkit.modal('#scriptentry', {
            escClose: false,
            bgClose: false,
            stack: true,
        }).show();
    }

    /**
     * Cancel script editing
     */
    cancelScript() {
        if (this.editorchanged) {
            UIkit.modal.confirm('You have unsaved changes. Are you sure you wish to close the editor?', {stack: true}).then(
                () => { // ok
                    this.destroyEditor();
                },
                () => { // cancel
                    this.showEditor();
                }
            ).catch(e => {
                console.log(e);
            });
            return;
        }
        this.destroyEditor();
    }

    /**
     * Save script as base64 encoded data into the current element model
     */
    saveScript() {
        this.appelement.attributes.scriptcontent = btoa(this.editor.getValue());
        this.destroyEditor();
    }

    /**
     * Destroy the editor after saving/cancelling so its clear for the next element
     */
    destroyEditor() {
        this.editor.destroy();
        this.editor = null;
        this.editorchanged = null;
        UIkit.modal('#scriptentry').hide();
    }
}
