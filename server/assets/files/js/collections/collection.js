/**
 * Represents a collection of items for assignment to the graph
 */
class Collection {
    // Timeout duration for waiting for objects to load
    TIMEOUT = 250;
    
    promisedKey = null;
    promisedValue = null;

    constructor(collectionType, attributes) {
        this.collectionType = collectionType.toLowerCase();
        this.objectType = this.collectionType.charAt(0).toUpperCase() + this.collectionType.slice(1);
        this.defaultAttrs = attributes;
        this.elements = {};
        this.object = null;
    }

    // Get all collection elements from the server
    load() {
        if (typeof(joint.shapes.container[this.objectType] === 'undefined')) {
            $.getScript(
                '/static/js/collections/' + this.collectionType + '.js',
                function() {
                    this.object = (Function('return new ' + this.objectType))();
                    this.object.setupEvents();
                    this.getElements();
                }.bind(this)
            );
        }
    }
    
    isGroupType() {
        return this.object.groupType;
    }
    
    getElements() {
        var elements = [];
        if (this.collectionType == 'link') {
            this.elements = this.object.getElements();
            return;
        }
        $.get('/api/v1/collections/' + this.collectionType, function (data) {
            elements = data.message;
            elements.sort(function (a, b) {
                return a.toLowerCase().localeCompare(b.toLowerCase());
            });
            for (var i = 0; i < elements.length; i++) {
                var element = elements[i].split('.')[0];
                this.elements[element] = null;
            }
            this.waitFor(this.callback.bind(this));
        }.bind(this));
    }

    clone(what) {
        return this.get(what).clone();
    }

    get(what) {
        if (Object.keys(this.elements).length != 0) {
            return this.elements[what];
        }
        this.promisedKey = what;
        return this.waitFor(this.promisedGet.bind(this));
    }

    promisedGet() {
        console.log("Sending back " + this.promisedKey);
        return this.get(this.promisedKey);
    }

    // Wait for the current object to be loaded
    waitFor(callback) {
        if(Object.keys(this.elements).length > 0) {
            return callback();
        }
        window.setTimeout(this.waitFor.bind(this), this.TIMEOUT, callback);
    }

    close() {
        return this.object.close();
    }
    
    attributes(view, event, x, y) {
        return this.object.attributes(view, event, x, y);
    }

    // Apply the collection template
    template() {
        var source = $('#' + this.collectionType + 'tpl').html();
        var template = Handlebars.compile(source);
        var html = template({
            id: this.collectionType + 'tpl',
            list: Object.keys(this.elements),
        });
        $('#' + this.collectionType + '-element-list').html(html);
    }

    // Load the assigned element type
    graphElement() {
        Object.keys(this.elements).forEach(function (element) {
            $.get('/static/img/' + this.collectionType + '/' + element + '.svg', function(data) {
                var attrs = this.defaultAttrs;
                attrs['element'] = element;
                attrs['attrs'] = {
                    '.body': {
                        'xlink:href': 'data:image/svg+xml;utf8,' + encodeURIComponent(new XMLSerializer().serializeToString(data.documentElement))
                    },
                }
                this.elements[element] = new joint.shapes.container[this.objectType](attrs);
            }.bind(this));
        }.bind(this));
    }
    
    // Sets up the graph element and templates the list onto the page
    callback() {
        this.graphElement();
        this.template();
    }
}
