/* Copyright 2021 The Tiyo authors
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

class Kubernetes extends Base {
    groupType = true;

    constructor() {
        super('kubernetes-element-list', 'containerProperties', collections.kubernetes)
    }

    attributes(view, event, x, y) {
        var element = $('.containerProperties');
        $('#containername').val(view.model.attributes.name);
        $('#containerscale').val(view.model.attributes.scale);
        element.css({
            "position": "absolute",
            "display": "block",
            "left": x,
            "top": y,
        });

        element.find('.done').click((e) => {
            view.model.attributes.name = $('#containername').val();
            view.model.attributes.scale = parseInt($('#containerscale').val(), 10);
            view.model.attr()['text'].text = $('#containername').val();

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

// Create the shape
joint.shapes.container.Kubernetes = joint.shapes.basic.Generic.extend({
    markup: '<g class="rotatable"><g class="scalable"><rect/></g><image/><text/></g>',
    defaults: joint.util.deepSupplement({
        type: 'container.Kubernetes',
        name: "",
        scale: 0,
        sourcetype: "",
        size: { width: 250, height: 250 },
        groupType: true,
        environment: [],
        attrs: {
            rect: { fill: '#fff', 'fill-opacity': '0', stroke: 'black', width: 100, height: 60 },
            text: { 'font-size': 14, text: '', 'ref-x': .5, 'ref-y': -15, ref: 'rect', 'y-alignment': 'top', 'x-alignment': 'middle', fill: 'black' },
            image: { 'ref-x': -15, 'ref-y': -15, ref: 'rect', width: 40, height: 40 },
        }
    }, joint.shapes.basic.Generic.prototype.defaults)
});

//# sourceURL=/static/js/collections/kubernetes.js
