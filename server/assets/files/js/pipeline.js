/* Copyright 2021 The Tiyo authors
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

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
    lastStatus = null;
    toolsTimeout = null;
    toolsId = null;

    gauges = {
        pipelineCpu: {
            label: "CPU",
            gauge: null,
        },
        pipelineMemory: {
            label: "Memory",
            gauge: null,
        },
        availableCpu: {
            label: "CPU",
            gauge: null,
        },
        availableMemory: {
            label: "Memory",
            gauge: null,
        },
    };

    constructor() {
        this.graph = new joint.dia.Graph;
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
        this.makeGauges();
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
            put('pipeline', null, title, btoa(JSON.stringify(this.graph.toJSON())));
            createFileStore(title);
            Cookies.set('pipeline', title);
            this.pipeline = title;
            success("Pipeline saved");
        }

        if (!this.autoSave) {
            this.autoSave = setInterval(this.save.bind(this), 60000);
        }
    }

    load() {
        if (router.lastResolved()[0].url != "pipeline") {
            return;
        }
        this.pipeline = Cookies.get('pipeline');
        if (this.pipeline !== "") {
            $.get('/api/v1/bucket/pipeline/' + encodeURI(this.pipeline),
                (data, status) => {
                    if (data && data.code == 200) {
                        this.graph.fromJSON(JSON.parse(atob(data.message)));
                        $('.editable.pipelinetitle').text(Cookies.get('pipeline'));
                        this.status();
                        this.makeGauges();
                    }
                }
            ).fail(
                (error) => {
                    Cookies.remove('pipeline');
                    handleError(error);
                }
            );
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

        $.post("/api/v1/execute",
            JSON.stringify({
                pipeline: this.pipeline,
            }),
            (data, status) => {
                this.status();
            }
        ).fail((error) => {
            handleError(error)
        });
    }

    isExecuting() {
        return this.executing;
    }

    status() {
        if (this.statusCheck == null) {
            this.statusCheck = setInterval(this.status.bind(this), this.INTERVAL);
        }

        $.get("/api/v1/status/" + encodeURI(this.pipeline),
            (data) => {
                $("#execute").prop('disabled', true);
                this.lastStatus = data.message;

                var status = data["message"]["status"];
                var groups = data.message.groups;
                var nodes = data.message.nodes;

                var color = '#000';
                for (var key in groups) {
                    var group = groups[key];
                    var groupContainer = this.graph.getCell(key);
                    var groupContainerView = this.paper.findViewByModel(groupContainer);

                    switch (group.state) {
                        case 'Failed':
                            color = '#FF0000';
                            break;
                        case 'Ready':
                        case 'Running':
                            color = '#0000FF';
                            break;
                        case 'Busy':
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
            }
        ).fail((error) => {
            handleError(error)
        });
    }

    podStatus(group, containerKeys, expected) {
        var containers = {};
        for (var i = 0; i<containerKeys.length; i++) {
            containers[containerKeys[i]] = {
                Waiting:    0,
                Running:    0,
                Terminated: 0,
                Ready: 0,
                Busy: 0,
            }
        };

        for (var podkey in group.pods) {
            var pod = group.pods[podkey];
            for (var container in pod.containers) {
                var id = pod.containers[container].id;
                try {
                containers[id][pod.containers[container].state] += 1;
                } catch (TypeError) {}
                if (pod.containers[container].state == 'Ready' || pod.containers[container].state == 'Busy') {
                    containers[id]["Running"] += 1
                }
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
        this.makeGauges();
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
            $('#message').find('p').html('Failed to create bucket');
        });
    }

    canvasSize() {
        var canvas = $('#paper-pipeline');
        var size = Object();
        size.w = canvas.width();
        size.h = canvas.height();
        return size;
    }


    setupEvents() {
        $('#paper-pipeline').mousemove(function(event) {
            this.drag(event)
        }.bind(this));

        $(window).resize(() => {
            const canvas = this.canvasSize();
            this.paper.setDimensions(canvas.w, canvas.h);
        });

        this.paper.on({
            'blank:mousewheel':    (event, x, y, delta) => { this.blankMouseWheel(event, x, y, delta); },
            'blank:pointerdown':   (event, x, y) => { this.blankPointerDown(event, x, y); },
            'blank:pointerup':     (event, x, y) => { this.blankPointerUp(event, x, y); },

            'link:contextmenu':    (view, event, x, y) => { this.linkRightClick(view, event, x, y); },

            'element:contextmenu': (view, event, x, y) => { this.elementRightClick(view, event, x, y); },
            'element:mouseover':   (view, event, x, y) => { this.elementMouseOver(view, event, x, y); },
            'element:mouseleave':  (view, event, x, y) => { this.elementMouseLeave(view, event, x, y); },
            'element:mouseenter':  (view, event, x, y) => { this.elementMouseEnter(view, event, x, y) },
            'element:mousewheel':  (view, event, x, y, delta) => { this.blankMouseWheel(event, x, y, delta); },
            'element:pointerup':   (view, event, x, y) => { this.elementPointerUp(view, event, x, y); },

            'cell:pointerdown':    (view, event, x, y) => { this.cellPointerDown(view); },
            'cell:pointerup':      (view, event, x, y) => { this.cellPointerUp(view); },
        });
    }

    drag(event) {
        if (this.dragging) {
            const scale = this.paper.scale();
            var x = event.offsetX - (this.dragStartPosition.x * scale.sx)
            var y = event.offsetY - (this.dragStartPosition.y * scale.sy)
            this.paper.translate(x, y);
        }
    }

    blankMouseWheel(event, x, y, delta) {
        const scale = this.paper.scale();

        var minScale = 0.25
        var maxScale = 2
        var e = event.originalEvent;
        delta = delta * minScale;

        var offsetX = (e.offsetX || e.clientX - $(this).offset().left);
        var offsetY = (e.offsetY || e.clientY - $(this).offset().top);
        var p = this.offsetToLocalPoint(offsetX, offsetY);
        var newScale = scale.sx + delta;
        if (newScale >= minScale && newScale <= maxScale) {
            this.paper.translate(0, 0);
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
        var modelId = view.model.id;
        if (this.toolsTimeout != null && this.toolsId == modelId) {
            clearTimeout(this.toolsTimeout);
        }
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
        this.toolsId = view.model.id;
        this.toolsTimeout = setTimeout(function(v) {
            v.removeTools();
            this.toolsTimeout = null;
            this.toolsId = null;
        }.bind(this), 500, view);
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
        // dragEvent from global space
        return this.offsetToLocalPoint(
            dragEvent.offsetX || dragEvent.layerX,
            dragEvent.offsetY || dragEvent.layerY
        )
    }

    offsetToLocalPoint(x, y) {
        var svgPoint = this.paper.svg.createSVGPoint();
        svgPoint.x = x;
        svgPoint.y = y;
        return svgPoint.matrixTransform(this.paper.viewport.getCTM().inverse());
    }

    totalCpu() {
        if (this.lastStatus == null) {
            return '0';
        }
        var cpu = 0;
        for (var node in this.lastStatus.nodes) {
            node = this.lastStatus.nodes[node];
            cpu += parseInt(node.cpucapacity);
        }
        return cpu;
    }

    availableCpu() {
        if (this.lastStatus == null) {
            return 0.0001;
        }
        var cpu = 0;
        for (var node in this.lastStatus.nodes) {
            node = this.lastStatus.nodes[node];
            cpu += parseInt(node.cpurequests);
        }
        cpu = (cpu / this.totalCpu()) * 100;
        if (cpu == 0) { cpu = 0.0001; } else if (cpu >= 100) { cpu = 99.9999; }
        return  cpu;
    }

    parseCpu(cpu) {
        var num = parseFloat(cpu) * 1000;
        var ident = cpu.replace(/[0-9.]/g, '').trim().toLowerCase().slice(0, 1);
        if (ident == "m") {
            num /= 1000;
        }
        return num;
    }

    totalMemory() {
        if (this.lastStatus == null) {
            return 1;
        }
        var mem = 0;
        for (var node in this.lastStatus.nodes) {
            node = this.lastStatus.nodes[node];
            mem += parseInt(node.memorycapacity);
        }
        return mem;
    }

    availableMemory() {
        if (this.lastStatus == null) {
            return 0.0001;
        }
        var mem = 0;
        for (var node in this.lastStatus.nodes) {
            node = this.lastStatus.nodes[node];
            mem += parseInt(node.memoryrequests);
        }
        mem = (mem / this.totalMemory()) * 100;
        if (mem == 0) { mem = 0.0001; } else if (mem >= 100) { mem = 99.9999; }
        return mem;
    }

    parseMemory(mem) {
        var SI = ['b', 'kb', 'mb', 'gb', 'tb', 'pb', 'eb', 'zb', 'yb'];
        var IEC = ['b', 'ki', 'mi', 'gi', 'ti', 'pi', 'ei', 'zi', 'yi'];

        var num = parseFloat(mem);
        var ident = mem.replace(/[0-9.]/g, '').trim().toLowerCase().slice(0, 2);
        ident = ident == 'by' ? 'b' : ident;
        var size, multiplier;
        if (ident == "") {
            return num;
        }

        if ((size = SI.indexOf(ident)) != -1) {
            multiplier = 1024;
        } else if((size = IEC.indexOf(ident)) != -1) {
            multiplier = 1000;
        } else {
            ident = ident + "b";
            size = SI.indexOf(ident);
            multiplier = 1024;
        }
        return num * Math.pow(multiplier, (size +1));
    }

    requiredResources() {
        var cpu = 0;
        var mem = 0;
        this.graph.getCells().forEach((cell) => {
            if (cell.attributes.type == 'container.Kubernetes') {
                var scale = cell.attributes.scale;
                if (Object.keys(cell.attributes).includes("embeds")) {
                    for (var i=0; i<cell.attributes.embeds.length; i++) {
                        var id = cell.attributes.embeds[i];
                        this.graph.getCells().forEach((element) => {
                            if (element.attributes.id == id && element.attributes.type == 'container.Container') {
                                cpu += (this.parseCpu(element.attributes.cpu) * scale);
                                mem += (this.parseMemory(element.attributes.memory) * scale);
                            }
                        });
                    }
                }
            }
        });
        cpu = (cpu / this.totalCpu()) * 100;
        mem = (mem / this.totalMemory()) * 100

        if (cpu == 0) { cpu = 0.0001; } else if (cpu >= 100) { cpu = 99.9999; }
        if (mem == 0) { mem = 0.0001; } else if (mem >= 100) { mem = 99.9999; }
        return {
            cpu: cpu,
            mem: mem,
        }
    }

    /**
     * Gauge functionality
     */
    makeGauges() {
        var required = this.requiredResources();
        var rangeLabel = ['0', '100']
        var arcColors = ['rgb(44, 151, 222)', 'lightgray']
        Object.keys(this.gauges).forEach((gauge) => {
            var value;
            switch (gauge) {
                case 'pipelineCpu':
                    value = required.cpu;
                    break;
                case 'pipelineMemory':
                    value = required.mem;
                    break;
                case 'availableCpu':
                    value = this.availableCpu();
                    rangeLabel = ['100', '0'];
                    arcColors = ['lightgray', 'rgb(44, 151, 222)'];
                    break;
                case 'availableMemory':
                    value = this.availableMemory();
                    rangeLabel = ['100', '0'];
                    arcColors = ['lightgray', 'rgb(44, 151, 222)'];
                    break;
            }
            if (typeof(value) === 'undefined' || isNaN(value)) {
                value = 0.0001
            }
            if (this.gauges[gauge].gauge != null) {
                this.gauges[gauge].gauge.removeGauge()
            }
            var element = document.querySelector('#' + gauge);
            this.gauges[gauge].gauge = GaugeChart.gaugeChart(element, 150, {
                hasNeedle: false,
                needleColor: 'gray',
                needleUpdateSpeed: 1000,
                arcColors: arcColors,
                arcDelimiters: [value],
                rangeLabel: rangeLabel,
                centralLabel: this.gauges[gauge].label,
            });
            this.gauges[gauge].gauge.updateNeedle(value);
        });
    }


    showEnvironment() {
        this.editor = ace.edit("environment-content");
        this.editor.setTheme("ace/theme/xcode");

        var attributes = this.appelement.attributes;
        if (!Object.keys(attributes).includes("environment")) {
            attributes["environment"] = [];
        }

        if (this.appelement.attributes.environment.length > 0) {
            this.editor.session.setValue(this.appelement.attributes.environment.join("\n"));
            this.editor.session.$modified = false;
        }

        this.editor.session.on('change', function(delta) {
            this.editorchanged = delta;
        }.bind(this));
        this.editor.session.setMode('ace/mode/sh');

        UIkit.modal('#environment', {
            escClose: false,
            bgClose: false,
            stack: true,
        }).show();
    }

    /**
     * Cancel environment editing
     */
    cancelEnvironment() {
        if (this.editorchanged) {
            UIkit.modal.confirm(
                'You have unsaved changes. Are you sure you wish to close the editor?',
                {stack: true}
            ).then(
                () => { // ok
                    this.destroyEditor();
                },
                () => { // cancel
                    this.showEnvironment();
                }
            ).catch(e => {});
            return;
        }
        this.destroyEditor();
    }

    /**
     * Save script as base64 encoded data into the current element model
     */
    saveEnvironment() {
        this.appelement.attributes.environment = this.editor.getValue().split("\n");
        this.destroyEditor();
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

        var details = this.appelement.attributes.gitrepo

        $('#gitrepo').val(details.repo)
        $('#gitbranch').val(details.branch)
        $('#gituser').val(details.username)
        $('#gitpass').val(details.password)
        $('#gitentry').val(details.entrypoint)

        this.editor.session.on('change', function(delta) {
            this.editorchanged = delta;
        }.bind(this));

        var mode = this.appelement.attributes.element;
        if (!this.appelement.attributes.custom) {
            mode = this.appelement.attributes.command.split(" ")[0];
        }

        // This needs to be more dynamic / extensible
        var supportedLanguages = [
            'golang', 'groovy', 'javascript', 'perl', 'python', 'php',
            'r', 'sh', 'json', 'dockerfile', 'ruby', 'julia'
        ];
        if (!supportedLanguages.includes(mode)) {
            mode = 'sh';
        }

        this.editor.session.setMode('ace/mode/' + mode);

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
            UIkit.modal.confirm(
                'You have unsaved changes. Are you sure you wish to close the editor?',
                {stack: true}
            ).then(
                () => { // ok
                    this.destroyEditor();
                },
                () => { // cancel
                    this.showEditor();
                }
            ).catch(e => {});
            return;
        }
        this.destroyEditor();
    }

    /**
     * Save script as base64 encoded data into the current element model
     */
    saveScript() {
        this.appelement.attributes.scriptcontent = btoa(this.editor.getValue());
        var hashed = this.appelement.attributes.gitrepo.password;
        this.appelement.attributes.gitrepo = {
            repo: $('#gitrepo').val(),
            branch: $('#gitbranch').val(),
            username: $('#gituser').val(),
            entrypoint: $('#gitentry').val(),
            password: hashed,
        }

        var newpass = $('#gitpass').val();
        if (newpass != hashed) {
            $.post("/api/v1/encrypt", JSON.stringify({
                value: newpass,
            }), (data, status) => {
                this.appelement.attributes.gitrepo.password = data.message;
            }).fail((error) => {
                handleError(error)
            })
        }
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
        UIkit.modal('#environment').hide();
        UIkit.modal('#credentials').hide();
    }
}
