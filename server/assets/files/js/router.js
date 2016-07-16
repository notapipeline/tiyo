router = new Navigo();
var pages = ["dashboard", "buckets", "scan", "pipeline"];

pages.forEach(function _(v, i, a) {
  var page = v == "dashboard" ? "/" : "/" + v;
  router.on(page, function() {
    if (v === "") {
        loadBucketTable();
    } else if (v == "pipeline") {
        loadPipeline();
        loadApplications();
    } else if (v == "scan") {
        scan();
    }
    pages.forEach(function x(b, c, d){
        $('#' + b).hide();
    });
    savePipeline();
    $('#' + v).show();
  });
});

