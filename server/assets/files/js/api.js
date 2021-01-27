jQuery.each( [ "post", "put", "delete" ], function( i, method ) {
    jQuery[ method ] = function( url, data, callback, type ) {
        if ( jQuery.isFunction( data ) ) {
            type = type || callback;
            callback = data;
            data = undefined;
        }

        if (typeof(type) === 'undefined') {
            type = 'json';
            contentType = 'application/json';
        }
        return jQuery.ajax({
            url: url,
            type: method,
            contentType: contentType,
            dataType: type,
            data: data,
            success: callback
        });
    };
});

function createPipeline(name)
{
    console.log(name);
}

/**
 * deprecating
 */
function doEdit(key)
{
    b = $('#pbucket').val();
    router.navigate('/form');
    get(b, key);

    $('#bucket').val(b);
    $('#key').val(key);
}

function doPrefixScan(bucket)
{
    $('#pbucket').val(bucket);
    $('#pkey').val("");
    Cookies.set('bucket', bucket);

    scan();
    router.navigate('/scan');
}

function get(bucket, key)
{
    if (typeof(bucket) == "undefined") {
        bucket = $("#bucket").val();
    }

    if (typeof(key) == "undefined") {
        key = $("#key").val();
    }
    $.get(
        "/api/v1/bucket/" + bucket + "/" + key,
        function(data){
            if(data.result == "OK") {
                $('#value').val(data.message);
            }
        }
    );
}

function createBucket(redirect=true)
{
    if (router.lastResolved()[0].url == "/scan") {
        createChild(Cookies.get('bucket'), $('#bucket').val());
        return;
    }

    $.post("/api/v1/bucket", {
        bucket: $('#bucket').val().trim()
    }, function(data, status) {
        Cookies.set('bucket', $('#bucket').val());
        if (redirect) {
            window.location.assign('/scan');
        }
    }).fail(function(e) {
        $('#message').addClass('uk-alert-danger');
        console.log(e);
        $('#message').find('p').html('Failed to create bucket');
    });
}

function createChild(parentBucket, childName, redirect=true)
{
    $.post("/api/v1/bucket", JSON.stringify({
        bucket: parentBucket.trim(),
        child: childName.trim(),
    }), function(data, status) {
        Cookies.set('bucket', $('#bucket').val());
        if (redirect) {
            window.location.assign('/scan');
        }
    }).fail(function(e) {
        $('#message').addClass('uk-alert-danger');
        console.log(e);
        $('#message').find('p').html('Failed to create bucket');
    });
}

function createFileStore(name)
{
    createChild('files', name.toLowerCase().replaceAll(" ", "_").trim(), false);
}

function deleteBucket()
{
    $.delete("/api/v1/bucket/" + $('#bucket').val());
}

function deleteKey(key)
{
    if (typeof(key) == "undefined") {
        key = $("#key").val();
    }
    $.delete("/api/v1/bucket/" + $('#pbucket').val() + "/" + key);
}

function doDelete(key)
{
    if (confirm("Delete?")) {
        deleteKey(key);
    }
    window.setTimeout(scan, 1000);
}

function put(b, c, k, v) {
    if (typeof(b) === 'undefined') {
        b = $('#bucket').val();
    }

    if (typeof(c) === 'undefined' || c === null) {
        c = "";
        if (b.includes('/')) {
            var parts = b.split('/')
            b = parts[0];
            c = parts[1];
        }
    }

    if (typeof(k) === 'undefined') {
        k = $('#key').val();
    }
    if (typeof(v) === 'undefined') {
        v = $('#value').val();
    }

    $.put("/api/v1/bucket", JSON.stringify({ bucket: b, child: c, key: k, value: v }));
}

function scan(v) {
    $('#pfs').html("");
    var source = $('#exploretpl').html();
    var template = Handlebars.compile(source);

    var bucket = $('#pbucket').val();
    if (bucket === "") {
        bucket = Cookies.get('bucket');
        $('#pbucket').val(bucket);
    }

    if (typeof(v) !== 'undefined') {
        bucket = bucket + "/" + v;
        $('#pbucket').val(bucket);
    }
    var url = "/api/v1/scan/" + bucket;
    if ($('#pkey').val() !== "") {
        url = url + "/" + $('#pkey').val();
    }

    $.get(url, function(data) {
        var html    = template({
            id: "explore-"+$('#pbucket').val(),
            buckets: data.message.buckets,
            keys: data.message.keys,
            bucketlen: data.message.bucketlen,
            keylen: data.message.keylen
        });
        $('#pfs').html(html);
        waitForEl('td.editable', editableElements);
    });
}
