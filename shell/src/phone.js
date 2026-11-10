// A phone is a touch screen: the CSS pointer queries call a phone with a stylus a fine pointer,
// so they can't tell us. A phone shows its tabs as favicons under the toolbar, with the full
// list as a drawer, so the tab layout setting doesn't apply there.
export const phone = navigator.maxTouchPoints > 0;
