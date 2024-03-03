"use strict";

let messages;

htmx.onLoad(() => {
	messages = document.getElementById("messages");
	if (!messages) {
		return;
	}
	messages.scrollTop = messages.scrollHeight;
});

function sendMessage(event) {
	const input = document.getElementById("input-form");

	messages.scrollTop = messages.scrollHeight;
	input.focus();
	event.currentTarget.reset();
}
