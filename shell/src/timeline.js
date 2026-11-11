// A page's way onto the timeline, the app's one record of what happened and when
// (docs/logging.md). Entries go to Go over the page's bridge, each with the page's own
// time, performance.now(), which Go places on the app's clock: it knows when this
// document's clock started. So an entry says when the thing happened, however long its
// message waits.
//
//   const { mark, begin, end, warn } = timeline(channel, send);
//   mark('shell.mounted');             a moment
//   begin('settings.open');            a span's start; whoever sees its end calls end
//   warn('suggestions came for a query nobody made');

export function timeline(page, send) {
  const record = (kind, name, text, at) => send({ t: 'timeline', kind, name, text, at: at ?? performance.now() });
  const mark = (name, at) => record('mark', name, '', at);
  const end = (name, at) => record('end', name, '', at);
  return {
    mark,
    begin: (name) => record('begin', name, '', undefined),
    end,
    info: (text) => record('info', '', text, undefined),
    warn: (text) => record('warn', '', text, undefined),
    error: (text) => record('error', '', text, undefined),

    // painted calls then once the renderer has drawn what the page looks like now: after
    // the next frame's style, layout and paint. The frame reaches the screen a little
    // later, and nothing tells a page when; only its first paint has that time.
    painted(then) {
      requestAnimationFrame(() => {
        const after = new MessageChannel();
        after.port1.onmessage = () => then();
        after.port2.postMessage(0);
      });
    },

    // firstPaint marks <page>.first_paint, when the renderer first drew the page's
    // content, and <page>.on_screen, when that frame reached the screen, then calls
    // then with that time.
    firstPaint(then) {
      new PerformanceObserver((list, observer) => {
        const paint = list.getEntriesByName('first-contentful-paint')[0];
        if (!paint) return;
        observer.disconnect();
        const onScreen = paint.presentationTime ?? paint.startTime;
        mark(`${page}.first_paint`, paint.paintTime);
        mark(`${page}.on_screen`, onScreen);
        then?.(onScreen);
      }).observe({ type: 'paint', buffered: true });
    },
  };
}
