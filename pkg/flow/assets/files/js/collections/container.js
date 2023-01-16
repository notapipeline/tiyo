/* Copyright 2021 The Tiyo authors
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

class Container extends Base {
    groupType = false;

    constructor() {
        super('container-element-list', 'applicationProperties', collections.container);
    }

    attributes(view, event, x, y) {
        var element = $('.applicationProperties');
        element.find('h4').text('application properties');

        $('#appname').prop('disabled', false);
        $('#appname').val(view.model.attr()['.label'].text);
        if (view.model.attr()['.label'].text) {
            $('#appname').prop('disabled', true);
        }

        if (view.model.attributes.command == "") {
            view.model.attributes.command = view.model.attr()['.label'].text
        }

        $('#appautostart').prop('checked', view.model.attributes.autostart);
        $('#appcmd').val(view.model.attributes.command);
        $('#appargs').val(view.model.attributes.arguments);
        $('#appversion').val(view.model.attributes.version);
        $('#apptimeout').val(view.model.attributes.timeout);
        $('#appexposeport').val(view.model.attributes.exposeport);
        $('#appisudp').prop('checked', view.model.attributes.isudp);

        $('#appcpu').val(view.model.attributes.cpu);
        $('#appmemory').val(view.model.attributes.memory);

        $('#appscript').prop('checked', view.model.attributes.script);

        element.css({
            "position": "absolute",
            "display": "block",
            "left": x,
            "top": y,
        });

        element.find('.done').click((e) => {
            if (!view.model) {
                return;
            }

            view.model.attr()['.label'].text = $('#appname').val();
            view.model.attributes.autostart = $('#appautostart').prop('checked');
            view.model.attributes.name = $('#appname').val();
            view.model.attributes.command = $('#appcmd').val();
            view.model.attributes.arguments = $('#appargs').val();
            view.model.attributes.version = $('#appversion').val();

            view.model.attributes.timeout = parseInt($('#apptimeout').val(), 10);
            view.model.attributes.script = $('#appscript').prop('checked');

            view.model.attributes.cpu = $('#appcpu').val();
            view.model.attributes.memory = $('#appmemory').val();

            view.model.attributes.exposeport = parseInt($('#appexposeport').val());
            view.model.attributes.isudp = $('#appisudp').prop('checked');
            joint.dia.ElementView.prototype.render.apply(view, arguments);
            element.css({
                "display": "none",
            });
            pipeline.save();
            element.find('.done').off('click');
        });

        element.find('.cancel').click((e) => {
            element.css({
                "display": "none",
            });
            element.find('.cancel').off('click');
        });
    }
}

joint.shapes.container.Container = joint.shapes.devs.Model.extend({
    defaults: joint.util.deepSupplement({
        markup: '<g class="rotatable"><g class="scalable"><image class="body"/>'+
            '</g><text class="label"/><rect class="progress" /><g class="inPorts"/>'+
            '<g class="outPorts"/></g>',
        type: 'container.Container',
        perpendicularLinks: true,

        autostart: false,
        name: "",
        element: "",
        command: "",
        arguments: "",
        script: false,
        scriptcontent: "",
        custom: false,
        timeout: 15,
        existing: false,
        exposeport: -1,
        isudp: false,
        environment: [],
        sourcetype: "",

        cpu: "500m",
        memory: "256Mi",

        gitrepo: {
            repo: "",
            branch: "",
            username: "",
            password: "",
            entrypoint: "",
        },

        position: { x: 50, y: 50 },
        size: { width: 50, height: 50 },
        inPorts: ['a', 'b', 'c'],
        outPorts: ['o'],
        attrs: {
            '.label': {
                text: 'Container',
                'ref-x': 0.5,
                'ref-y': 50,
                'font-size': '10pt',
            },
            '.progress': {
                'ref-x': 0,
                'ref-y': 70,
                'ref-height': '8%',
                'ref-width': '0%',
                'fill': '#ff0000',
            }
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

//# sourceURL=/static/js/collections/container.js
