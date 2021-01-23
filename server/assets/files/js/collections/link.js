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

class Link {
    $fileProperties = $(
        '<div class="fileProperties linkProperties properties">'+
        '<h4></h4>'+
        '<form>'+
        '  <table>'+
        '    <tr>'+
        '      <td><label for="filepath">Path</label></td>'+
        '      <td><input id="filepath" value="" /></td>'+
        '    </tr>'+
        '    <tr>'+
        '      <td><label for="filepattern">Pattern</label></td>'+
        '      <td><input id="filepattern" value="" /></td>'+
        '    </tr>'+
        '    <tr>'+
        '      <td><label for="filewatch">Watch</label></td>'+
        '      <td><input type="checkbox" id="filewatch" /></td>'+
        '    </tr>'+
        '  </table>'+
        '  <div style="float: right;">'+
        '    <a class="uk-button-small cancel">cancel</a>'+
        '    <a class="uk-button-small uk-button-primary done">done</a>'+
        '  </div>'+
        '</form>'+
        '</div>'
    );

    $tcpProperties = $(
        '<div class="tcpProperties linkProperties properties">'+
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

    $socketProperties = $(
        '<div class="socketProperties linkProperties properties">'+
        '<h4></h4>'+
        '<form>'+
        '  <table>'+
        '    <tr>'+
        '      <td><label for="socketpath">Path</label></td>'+
        '      <td><input id="socketpath" value="" /></td>'+
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
        $('#paper-pipeline-holder').append(this.$tcpProperties);
        $('#paper-pipeline-holder').append(this.$socketProperties);
        $('#paper-pipeline-holder').append(this.$fileProperties);
    }

    getElements() {
        return {
            file: this.file(),
            tcp: this.tcp(),
            udp: this.udp(),
            socket: this.socket()
        };
    }

    setupEvents() {
        // links
        UIkit.util.on('#pipeline-link-list', 'start', (e) => {
            activeDragEvent = e;
            elem = e.detail[1];
            activeLink = collections.link.get(
                $(elem).find('p').text().toLowerCase()
            );
            document.getElementById('paper-pipeline-holder').addEventListener('pointermove', onDragging);
        });

        UIkit.util.on('#pipeline-link-list', 'stop', (e) => {
            if (!target) {
                return;
            }
            document.getElementById('paper-pipeline-holder').removeEventListener('pointermove', onDragging);
            target = null;
            elem = null;
        });
    }
    
    close() {
        $('.linkProperties').each( function (_, el) {
            $(el).find('.done').off('click');
            $(el).find('.cancel').off('click');
            $(el).css({
                'display': 'none',
            });
        });
    }
    
    /**
     * Show properties on right click against a link
     */
    attributes(view, event, x, y) {
        pipeline.closeAll();
        var attributes = view.model.attributes.attributes;
        var element = null;
        if (['tcp', 'udp'].includes(attributes.type)) {
            element = $('.tcpProperties');
            $('#tcpsourceport').val(attributes.source);
            $('#tcpdestport').val(attributes.dest);
            $('#tcpaddress').val(attributes.address);
        } else if (attributes.type == 'socket') {
            element = $('.socketProperties');
            $('#socketpath').val(attributes.path);
        } else if (attributes.type == 'file') {
            element = $('.fileProperties');
            $('#filepath').val(attributes.path);
            $('#filepattern').val(attributes.pattern);
            $('#filewatch').prop('checked', attributes.watch);
        }
        element.find('h4').text(attributes.type + ' properties');

        element.css({
            "position": "absolute",
            "display": "block",
            "left": event.offsetX,
            "top": event.offsetY,
        });

        element.find('.done').click((e) => {
            if (['tcp', 'udp'].includes(attributes.type)) {
                attributes.source = $('#tcpsourceport').val();
                attributes.dest = $('#tcpdestport').val();
                attributes.address = $('#tcpaddress').val();
            } else if (attributes.type == 'socket') {
                attributes.path = $('#socketpath').val();
            } else if (attributes.type == 'file') {
                attributes.path = $('#filepath').val();
                attributes.pattern = $('#filepattern').val();
                attributes.watch = $('#filewatch').prop('checked');
            }
            element.css({
                "display": "none",
            });
            pipeline.save();
            var done = element.find('.done');
            done.off('click');
            element = null;
            attributes = null;
        });

        element.find('.cancel').click((e) => {
            $(this.offsetParent).css({
                "display": "none",
            });
            $(this).off('click');
            element = null;
            attributes = null;
        });
    }
    
    tcp() {
        return new joint.dia.Link({
            attrs: {
                '.marker-target': {
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
    }
    
    udp() {
        return new joint.dia.Link({
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
    }
    
    socket() {
        return new joint.dia.Link({
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
    }
    
    file() {
        return new joint.dia.Link({
            attrs: {
                '.marker-target': { d: 'M 10 0 L 0 5 L 10 10 z' },
            },
            attributes: {
                type: "file",
                path: "",
                pattern: "",
                watch: false,
            },
        });
    }
}

//# sourceURL=/static/js/collections/links.js
