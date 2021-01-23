router = new Navigo('/');
var pages = ["dashboard", "buckets", "scan", "pipeline"];

pages.forEach(function _(v, i, a) {
  var page = v == "dashboard" ? "/" : "/" + v;
  router.on(page, function() {
    if (v === "") {
        loadBucketTable();
    } else if (v == "pipeline") {
        // pipeline uses file link as part of the paper constructor
        collections.link.waitFor(() => {
            pipeline = new Pipeline();
            pipeline.setupEvents();
            pipeline.load();
            pipeline.save();
        });
        loadApplications();
    } else if (v == "scan") {
        scan();
    }
    pages.forEach(function x(b, c, d){
        $('#' + b).hide();
    });
    $('#' + v).show();
  });
});

