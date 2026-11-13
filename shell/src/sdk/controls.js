// What a control is: something the user presses or types in. Buttons, links with an
// address, fields, selects, text areas, what calls itself a tab or a button, and anything
// with a tabindex, which takes focus. In a bar marked drag the controls stay the shell's
// and the rest of the bar moves the window. The shells' check reads this same selector,
// so a shell is held to the notion the SDK acts on (shell/e2e/shell-contract.cjs).
export const controls =
  'button, a[href], input, select, textarea, [contenteditable], [role="tab"], [role="button"], [tabindex]';
