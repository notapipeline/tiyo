/* Copyright 2021 The Tiyo authors
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

class Kubernetes {
    $containerProperties = $(
        '<div class="containerProperties properties">'+
        '<h4>Kubernetes set properties</h4>'+
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
        '    <tr>'+
        '      <td></td>'+
        '      <td><input type="button" value="environment" onclick="pipeline.showEnvironment()"></td>'+
        '    </tr>'+
        '  </table>'+
        '  <div style="float: right;">'+
        '    <a class="uk-button-small cancel">cancel</a>'+
        '    <a class="uk-button-small uk-button-primary done">done</a>'+
        '  </div>'+
        '</form>'+
        '</div>'
    );

    groupType = true;

    constructor() {
        $('#paper-pipeline-holder').append(this.$containerProperties);
    }

    setupEvents() {
        UIkit.util.on('#kubernetes-element-list', 'start', (e) => {
            elem = e.detail[1];
            document.getElementById('paper-pipeline-holder').addEventListener('pointermove', onDragging);
        });

        UIkit.util.on('#kubernetes-element-list', 'stop', (e) => {
            if (!target) {
                return;
            }
            document.getElementById('paper-pipeline-holder').removeEventListener('pointermove', onDragging);
            if ($(target)[0].nodeName == "svg" && $(target)[0].parentElement.id == "paper-pipeline") {
                var point = pipeline.getTransformPoint();
                var name = $(elem).find('img').attr('src').replace(/.*\//, '').split('.')[0];
                var container = collections.kubernetes.clone(name).position(
                    point.x, point.y
                ).attr({
                    text: {
                        text: name,
                    },
                    image: {
                        'xlink:href': $(elem).find('img').attr('src'),
                    },
                });
                container.attributes.name = name;
                container.attributes.settype = name;
                container.addTo(pipeline.graph);
            }
            target = null;
            elem = null;
        });
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
            pipeline.save();
            element.find('.done').off('click');
            joint.dia.ElementView.prototype.render.apply(view, arguments);
        });

        element.find('.cancel').click((e) => {
            element.css({
                "display": "none",
            });
            element.find('.cancel').off('click');
        });
    }

    close() {
        $('.containerProperties').find('.done').off('click');
        $('.containerProperties').find('.cancel').off('click');
        $('.containerProperties').css({
            'display': 'none',
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
        settype: "",
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
