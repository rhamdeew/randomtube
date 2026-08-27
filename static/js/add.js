(function () {
    'use strict';

    document.querySelectorAll('[data-job]').forEach(function (el) {
        var jobID = el.dataset.job;
        var interval = setInterval(function () {
            fetch('/add/job/' + jobID)
                .then(function (r) { return r.json(); })
                .then(function (data) {
                    if (data.status === 'done' || data.status === 'error') {
                        clearInterval(interval);
                        location.reload();
                    } else {
                        el.textContent = data.status + ' ' + data.imported;
                    }
                })
                .catch(function () { clearInterval(interval); });
        }, 2000);
    });
}());
