(function () {
    'use strict';

    var LANG = document.documentElement.lang === 'ru' ? 'ru' : 'en';
    var MSG = {
        en: {
            selectOne: 'Select at least one video',
            confirmDeleteMany: function (n) { return 'Delete ' + n + ' videos?'; },
            confirmDeleteOne: function (id) { return 'Delete video #' + id + '?'; }
        },
        ru: {
            selectOne: 'Выберите хотя бы одно видео',
            confirmDeleteMany: function (n) { return 'Удалить ' + n + ' видео?'; },
            confirmDeleteOne: function (id) { return 'Удалить видео #' + id + '?'; }
        }
    }[LANG];

    // Check-all checkbox
    var checkAll = document.getElementById('check-all');
    if (checkAll) {
        checkAll.addEventListener('change', function () {
            document.querySelectorAll('input[name="ids"]').forEach(function (cb) {
                cb.checked = checkAll.checked;
            });
        });
    }

    // Bulk action triggered from buttons
    window.bulkAction = function (action) {
        var checked = document.querySelectorAll('input[name="ids"]:checked');
        if (checked.length === 0) {
            alert(MSG.selectOne);
            return;
        }
        if (action === 'delete' && !confirm(MSG.confirmDeleteMany(checked.length))) return;
        document.getElementById('bulk-action').value = action;
        document.getElementById('bulk-form').submit();
    };

    // Single-row action (no checkbox needed)
    window.singleAction = function (action, id) {
        if (action === 'delete' && !confirm(MSG.confirmDeleteOne(id))) return;

        var form = document.getElementById('bulk-form');
        // temporarily uncheck all, check only this id
        document.querySelectorAll('input[name="ids"]').forEach(function (cb) {
            cb.checked = (parseInt(cb.value) === id);
        });
        document.getElementById('bulk-action').value = action;
        form.submit();
    };

    // Poll running import jobs
    document.querySelectorAll('[data-job]').forEach(function (el) {
        var jobID = el.dataset.job;
        var interval = setInterval(function () {
            fetch('/admin/import/job/' + jobID)
                .then(function (r) { return r.json(); })
                .then(function (data) {
                    if (data.status !== 'running') {
                        clearInterval(interval);
                        location.reload();
                    } else {
                        el.textContent = 'running ' + data.imported;
                    }
                })
                .catch(function () { clearInterval(interval); });
        }, 2000);
    });
}());
