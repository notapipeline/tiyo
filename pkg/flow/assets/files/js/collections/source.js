/* Copyright 2021 The Tiyo authors
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

class Source extends Base {
    groupType = false;

    constructor() {
        super('source-element-list', 'sourceProperties', collections.source)
    }

    attributes(view, event, x, y) {
        var element = $('.sourceProperties');
        $('#sourcename').val(view.model.attributes.name);
        element.css({
            "position": "absolute",
            "display": "block",
            "left": x,
            "top": y,
        });

        element.find('.done').click((e) => {
            view.model.attributes.name = $('#sourcename').val();
            view.model.attr()['.label'].text = $('#sourcename').val();

            element.css({
                "display": "none",
            });
            element.find('.done').off('click');
            joint.dia.ElementView.prototype.render.apply(view, arguments);
            pipeline.save();
        });

        element.find('.cancel').click((e) => {
            element.css({
                "display": "none",
            });
            element.find('.cancel').off('click');
        });
    }
}

joint.shapes.container.Source = joint.shapes.devs.Model.extend({
    defaults: joint.util.deepSupplement({
        markup: '<g class="rotatable"><g class="scalable"><image class="body"/></g><text class="label"/><g class="inPorts"/><g class="outPorts"/></g>',
        type: 'container.Source',
        perpendicularLinks: true,

        name: "",
        sourcetype: "",
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

//# sourceURL=/static/js/collections/source.js
