/* Copyright 2021 The Tiyo authors
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

jQuery.each( [ "post", "put", "delete" ], function( i, method ) {
    jQuery[ method ] = function( url, data, callback, type ) {
        if ( jQuery.isFunction( data ) ) {
            type = type || callback;
            callback = data;
            data = undefined;
        }

        var contentType = type;
        if (typeof(type) === 'undefined') {
            type = 'json';
            contentType = 'application/json; charset=utf-8';
        }
        return jQuery.ajax({
            url: url,
            type: method,
            contentType: contentType,
            dataType: type,
            data: data,
            success: callback,
        });
    };
});

