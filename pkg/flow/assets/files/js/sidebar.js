 /* Copyright 2021 The Tiyo authors
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

class Sidebar {
    page = null;
    element = null;
    components = null

    constructor(page, element) {
        this.page = page;
        this.element = $($(element).find('nav > ul')[0]);

        this._setup();
    }

    _setup() {
        // Get Sidebar components JSON
        // {
        //   "component": {
        //     "element": {
        //       "icon": /static/img/blah[/blah].svg,
        //       "schema": /schema/component/version,
        //       "group": api group,
        //       "version": api version,
        //       "kind": kubernetes singular kind
        //     }
        //     ...
        //   }
        //   ...
        // }
        $.getJSON('/api/v1/sidebar', function(components) {
            this.components = components;
            console.log(this.components);
            for (var key in this.components) {
                var value = this.components[key];
                this.element.append('<li><a class="uk-accordion-title">'+ key +'</a>' +
                    '<div class="uk-accordion-content uk-flex-middle uk-margin-bottom">' +
                    '<ul uk-sortable="handle: .' + key + '-element" class="uk-grid-stack uk-height-max-large ' +
                    'element-list" id="' + key + '-element-list"></ul></div></li>'
                );

                for (var k in value) {
                    $('#' + key + '-element-list').append('<div class="'+
                        k.replaceAll(".", "-") +'-block" style="clear: both;"><h5>'+k+'</h5></div>');
                    $(value[k]).each(function(i, v){
                        $('.' + k.replaceAll(".", "-") + '-block').append(
                            '<li class="uk-card uk-card-default uk-card-body ' +
                            key + '-element element-list-element">' + '<image src="' + v.icon + '" alt="' +
                            v.kind + '" uk-tooltip="' + v.kind + '" /></li>'
                        );
                    });
                }
            }
        }.bind(this));
    }

    find(what) {
        for (var key in this.components) {
            var value = this.components["key"];
            for (var k in value) {
                var v = value["k"];
                if (what == v.icon || what == v.kind) {
                    return v;
                }
            }
        }
        return null;
    }
}
