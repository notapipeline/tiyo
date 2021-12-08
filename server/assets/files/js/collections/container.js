/* Copyright 2021 The Tiyo authors
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

class Container {
    $appProperties = $(
        '<div class="applicationProperties properties">'+
        '  <h4>Container properties</h4>'+
        '  <form>'+
        '    <table>'+
        '      <tr>'+
        '        <td><label for="appautostart">autostart</label></td>'+
        '        <td>'+
        '          <input type="checkbox" id="appautostart" />'+
        '        </td>'+
        '      </tr>'+
        '      <tr>'+
        '        <td><label for="appname">name</label></td>'+
        '        <td><input id="appname" value="" /></td>'+
        '      </tr>'+
        '      <tr>'+
        '        <td><label for="appcmd">command</label></td>'+
        '        <td><input id="appcmd" value="" /></td>'+
        '      </tr>'+
        '      <tr>'+
        '        <td><label for="appargs">arguments</label></td>'+
        '        <td><input id="appargs" value=""></td>'+
        '      </tr>'+
        '      <tr>'+
        '        <td><label for="appversion">version</label></td>'+
        '        <td><input id="appversion" value=""></td>'+
        '      </tr>'+
        '      <tr>'+
        '        <td><label for="apptimeout">timeout</label></td>'+
        '        <td><input id="apptimeout" value=""></td>'+
        '      </tr>'+
        '      <tr>'+
        '        <td><label for="appexposeport">expose port</label></td>'+
        '        <td>'+
        '            <input id="appexposeport" value="" style="width:110px;">'+
        '            <span><label for="appisudp">UDP</label><input type="checkbox" id="appisudp" /></span>'+
        '        </td>'+
        '      </tr>'+
        '      <tr>'+
        '        <td><label for="appcpu">cpu</label></td>'+
        '        <td><input id="appcpu" value=""></td>'+
        '      </tr>'+
        '      <tr>'+
        '        <td><label for="appmemory">memory</label></td>'+
        '        <td><input id="appmemory" value=""></td>'+
        '      </tr>'+
        '      <tr>'+
        '        <td><label for="appscript">script</label></td>'+
        '        <td><input type="checkbox" id="appscript" />'+
        '            <input type="button" id="editappscript" value="edit" onclick="pipeline.showEditor()" />' +
        '            <input type="button" value="environment" onclick="pipeline.showEnvironment()" />' +
        '        </td>'+
        '      </tr>'+
        '    </table>'+
        '    <div style="float: right;">'+
        '      <a class="uk-button-small cancel">cancel</a>'+
        '      <a class="uk-button-small uk-button-primary done">done</a>'+
        '    </div>'+
        '  </form>'+
        '</div>'
    );

    groupType = false;

    constructor() {
        $('#paper-pipeline-holder').append(this.$appProperties);
    }

    setupEvents() {
        UIkit.util.on('#container-element-list', 'start', (e) => {
            elem = e.detail[1];
            document.getElementById('paper-pipeline-holder').addEventListener('pointermove', onDragging);
        });


        UIkit.util.on('#container-element-list', 'stop', (e) => {
            if (!target) {
                return;
            }
            document.getElementById('paper-pipeline-holder').removeEventListener('pointermove', onDragging);
            var name = $(elem).find('img').attr('src').replace(/.*\//, '').split('.')[0];
            var point = pipeline.getTransformPoint();
            var cell = collections.container.clone(name).position(
                point.x, point.y
            ).attr(
                '.label/text', elem.textContent.trim()
            );
            pipeline.checkEmbed(cell, point);
            target = null;
            elem = null;
        });
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

    close() {
        $('.applicationProperties').find('.done').off('click');
        $('.applicationProperties').find('.cancel').off('click');
        $('.applicationProperties').css({
            'display': 'none',
        });
    }
}

joint.shapes.container.Container = joint.shapes.devs.Model.extend({
    defaults: joint.util.deepSupplement({
        markup: '<g class="rotatable"><g class="scalable"><image class="body"/></g><text class="label"/><rect class="progress" /><g class="inPorts"/><g class="outPorts"/></g>',
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
                text: 'Model',
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
