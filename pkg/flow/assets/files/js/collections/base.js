class Base {
    $properties = null;
    $clonable = null;
    $id = "";
    $className = "";
    $type = null;

    constructor(id, className, clonable) {
        this.$id = id;
        this.$className = className;
        this.$clonable = clonable;
    }

    attributes() {
        throw "Abstract method attributes not implemented";
    }

    eventsCallback(elem, source) {
        throw "Abstract method attributes not implemented";
    }

    getPropertiesForm(objectType) {
        $.get('/objectforms/' + objectType, function (data) {
            this.$properties = $(data.html)[0];
            $('#paper-pipeline-holder').append(this.$properties);
        });
    }

    attributesForm() {
        if (this.$properties == null) {
            this.getPropertiesForm(this.$type)
        }
    }

    setupEvents() {
        UIkit.util.on('#' + this.$id, 'start', (e) => {
            elem = e.detail[1];
            document.getElementById('paper-pipeline-holder').addEventListener('pointermove', onDragging);
        });

        UIkit.util.on('#' + this.$id, 'stop', (e) => {
            if (!target) {
                return;
            }
            document.getElementById('paper-pipeline-holder').removeEventListener('pointermove', onDragging);

            // does not work for embedding
            //if ($(target)[0].nodeName == "svg" && $(target)[0].parentElement.id == "paper-pipeline") {
            this.$type = $(elem).find('img').attr('src').replace(/.*\//, '').split('.')[0];
            var point = pipeline.getTransformPoint();
            var source = this.$clonable.clone(this.$type).position(
                point.x, point.y
            ).attr({
                '.label': {
                    text: this.$type,
                },
                text: {
                    text: this.$type,
                },
                image: {
                    'xlink:href': $(elem).find('img').attr('src'),
                },
            });
            try {
                this.eventsCallback(source);
            } catch {}

            source.attributes.name = this.$type;
            source.attributes.sourcetype = this.$type;
            pipeline.checkEmbed(source, point);
            //}
            target = null;
            elem = null;
        });
    }

    close() {
        $('.' + this.$className).find('.done').off('click');
        $('.' + this.$className).find('.cancel').off('click');
        $('.' + this.$className).remove();
    }
}
