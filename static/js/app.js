(function () {
    'use strict';

    if (!window.RT || !RT.currentID) return;

    var iframe = document.getElementById('ytplayer');
    var NOCOOKIE = 'https://www.youtube-nocookie.com';

    // Inject origin into iframe src so YouTube knows where to send postMessage events
    iframe.src = NOCOOKIE + '/embed/' + RT.currentID +
        '?autoplay=1&rel=0&enablejsapi=1&origin=' + encodeURIComponent(window.location.origin);

    iframe.addEventListener('load', function () {
        iframe.contentWindow.postMessage(
            JSON.stringify({ event: 'listening' }),
            NOCOOKIE
        );
    });

    window.addEventListener('message', function (e) {
        if (e.origin !== NOCOOKIE) return;
        try {
            var data = JSON.parse(e.data);
            // YouTube sends state changes in two formats
            var state = null;
            if (data.event === 'onStateChange') {
                state = data.info;
            } else if (data.event === 'infoDelivery' && data.info && data.info.playerState !== undefined) {
                state = data.info.playerState;
            }
            if (state === 0) { // 0 = ended
                fetchNext(RT.currentID);
            }
            // Error events
            var errCode = null;
            if (data.event === 'onError') {
                errCode = data.info;
            } else if (data.event === 'infoDelivery' && data.info && data.info.errorCode !== undefined) {
                errCode = data.info.errorCode;
            }
            // 100: not found/private, 101/150: embedding disabled
            if (errCode === 100 || errCode === 101 || errCode === 150) {
                reportAndNext(RT.currentID);
            }
        } catch (err) {}
    });

    function reportAndNext(youtubeID) {
        post('/report', { id: youtubeID, cat: RT.catCode }, function (data) {
            if (data && data.id) loadVideo(data.id, data.name);
        });
    }

    function fetchNext(currentID) {
        post('/next', { current: currentID, cat: RT.catCode }, function (data) {
            if (data && data.id) loadVideo(data.id, data.name);
        });
    }

    function loadVideo(id, name) {
        RT.currentID = id;
        // Use postMessage command to avoid full iframe reload
        iframe.contentWindow.postMessage(
            JSON.stringify({ event: 'command', func: 'loadVideoById', args: [id] }),
            NOCOOKIE
        );
        var nameEl = document.getElementById('video-name');
        if (nameEl) nameEl.textContent = name || '';
    }

    document.getElementById('btn-next').addEventListener('click', function () {
        fetchNext(RT.currentID);
    });

    document.getElementById('btn-like').addEventListener('click', function () {
        vote('like');
    });

    document.getElementById('btn-dislike').addEventListener('click', function () {
        vote('dislike');
    });

    function vote(button) {
        post('/vote', { id: RT.currentID, button: button }, function () {});
    }

    function post(url, data, cb) {
        var params = new URLSearchParams();
        for (var k in data) params.append(k, data[k]);

        fetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: params.toString()
        })
        .then(function (r) { return r.json(); })
        .then(cb)
        .catch(function () {});
    }
}());
