(function () {
    'use strict';

    if (!window.RT || !RT.currentID) return;

    var player = null;

    window.onYouTubeIframeAPIReady = function () {
        player = new YT.Player('ytplayer', {
            videoId: RT.currentID,
            playerVars: { autoplay: 1, rel: 0 },
            events: {
                onError: function (e) {
                    // 100: video not found/private, 101/150: embedding disabled
                    if (e.data === 100 || e.data === 101 || e.data === 150) {
                        reportAndNext(RT.currentID);
                    }
                },
                onStateChange: function (e) {
                    if (e.data === YT.PlayerState.ENDED) {
                        fetchNext(RT.currentID);
                    }
                }
            }
        });
        RT.player = player;
    };

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
        if (player && player.loadVideoById) {
            player.loadVideoById(id);
        }
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
        post('/vote', { id: RT.currentID, button: button }, function (data) {
            // silent – no UI feedback needed
        });
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
