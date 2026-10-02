(function () {
    'use strict';

    if (!window.RT || !RT.currentID) return;

    var iframe = document.getElementById('ytplayer');
    var HOSTS = {
        nocookie: 'https://www.youtube-nocookie.com',
        youtube: 'https://www.youtube.com'
    };
    var PLAYER_KEY = 'rt.player';
    var COPIED_TEXT = document.documentElement.lang === 'ru' ? 'Скопировано' : 'Copied';

    // Privacy-enhanced player by default; the regular one is an opt-in fallback
    // for users YouTube asks to sign in (sign-in doesn't work on nocookie).
    var playerMode = readPlayerMode();
    var playerOrigin = HOSTS[playerMode];

    function readPlayerMode() {
        try {
            return localStorage.getItem(PLAYER_KEY) === 'youtube' ? 'youtube' : 'nocookie';
        } catch (err) {
            return 'nocookie';
        }
    }

    function setPlayerMode(mode) {
        playerMode = mode;
        playerOrigin = HOSTS[mode];
        try {
            if (mode === 'youtube') localStorage.setItem(PLAYER_KEY, mode);
            else localStorage.removeItem(PLAYER_KEY);
        } catch (err) {}
        setIframeSrc(RT.currentID);
    }

    // Inject origin into iframe src so YouTube knows where to send postMessage events
    function setIframeSrc(id) {
        iframe.src = playerOrigin + '/embed/' + id +
            '?autoplay=1&rel=0&enablejsapi=1&origin=' + encodeURIComponent(window.location.origin);
    }
    setIframeSrc(RT.currentID);

    iframe.addEventListener('load', function () {
        iframe.contentWindow.postMessage(
            JSON.stringify({ event: 'listening' }),
            playerOrigin
        );
    });

    window.addEventListener('message', function (e) {
        if (e.origin !== playerOrigin) return;
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
            playerOrigin
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

    document.getElementById('btn-share').addEventListener('click', function () {
        share();
    });

    var modal = document.getElementById('player-modal');

    function openPlayerModal() {
        var blocks = modal.querySelectorAll('[data-mode]');
        for (var i = 0; i < blocks.length; i++) {
            blocks[i].classList.toggle('is-hidden', blocks[i].getAttribute('data-mode') !== playerMode);
        }
        modal.classList.add('is-active');
    }

    function closePlayerModal() {
        modal.classList.remove('is-active');
    }

    document.getElementById('btn-player-trouble').addEventListener('click', openPlayerModal);

    modal.addEventListener('click', function (e) {
        var sw = e.target.closest('[data-switch]');
        if (sw) {
            setPlayerMode(sw.getAttribute('data-switch'));
            closePlayerModal();
        } else if (e.target.closest('[data-close]')) {
            closePlayerModal();
        }
    });

    document.addEventListener('keydown', function (e) {
        if (e.key === 'Escape') closePlayerModal();
    });

    function share() {
        var url = window.location.origin + window.location.pathname + '?v=' + RT.currentID;
        var btn = document.getElementById('btn-share');
        var originalText = btn.textContent;

        function showCopied() {
            btn.textContent = COPIED_TEXT;
            setTimeout(function () { btn.textContent = originalText; }, 1500);
        }

        if (navigator.clipboard && navigator.clipboard.writeText) {
            navigator.clipboard.writeText(url).then(showCopied).catch(function () {});
        }
    }

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
