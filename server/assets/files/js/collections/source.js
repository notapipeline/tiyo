class Source {
    $sourceProperties = $(
        '<div class="sourceProperties properties">'+
        '<h4></h4>'+
        '<form>'+
        '  <table>'+
        '    <tr>'+
        '      <td><label for="sourcename">Name</label></td>'+
        '      <td><input id="sourcename" value="" /></td>'+
        '    </tr>'+
        '  </table>'+
        '  <div style="float: right;">'+
        '    <a class="uk-button-small cancel">cancel</a>'+
        '    <a class="uk-button-small uk-button-primary done">done</a>'+
        '  </div>'+
        '</form>'+
        '</div>'
    );

    groupType = false;

    constructor() {
        $('#paper-pipeline-holder').append(this.$sourceProperties);
    }

    setupEvents() {
        UIkit.util.on('#source-element-list', 'start', (e) => {
            elem = e.detail[1];
            document.getElementById('paper-pipeline-holder').addEventListener('pointermove', onDragging);
        });

        UIkit.util.on('#source-element-list', 'stop', (e) => {
            if (!target) {
                return;
            }
            document.getElementById('paper-pipeline-holder').removeEventListener('pointermove', onDragging);
            if ($(target)[0].nodeName == "svg" && $(target)[0].parentElement.id == "paper-pipeline") {
                var name = $(elem).find('img').attr('src').replace(/.*\//, '').split('.')[0];
                var point = pipeline.getTransformPoint();
                var source = collections.source.clone(name).position(
                    point.x, point.y
                ).attr(
                    '.label/text', elem.textContent.trim()
                );
                source.attributes.sourcetype = name;
                source.addTo(pipeline.graph);
            }
            target = null;
            elem = null;
        });
    }

    attributes(view, event, x, y) {
        var element = $('.sourceProperties');
        $('#sourcename').val(view.model.attributes.name);
        element.css({
            "position": "absolute",
            "display": "block",
            "left": event.offsetX,
            "top": event.offsetY,
        });

        element.find('.done').click((e) => {
            view.model.attributes.name = $('#sourcename').val();
            view.model.attr()['.label'].text = $('#sourcename').val();

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
        $('.sourceProperties').find('.done').off('click');
        $('.sourceProperties').find('.cancel').off('click');
        $('.sourceProperties').css({
            'display': 'none',
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
