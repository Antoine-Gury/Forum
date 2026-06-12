const discussionsList = document.getElementById("discussions-list");

function escapeHtml(text) {
    if (!text) return "";
    return text
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

function addDiscussionToDOM(discussion) {
    if (!discussionsList) return;

    const post = document.createElement("div");
    post.className = "post";
    post.innerHTML = `
        <div class="post-avatar">⚔</div>
        <div class="post-body">
            <h3><a href="/discussion?id=${discussion.ID}">${escapeHtml(discussion.Title)}</a></h3>
            <p>${escapeHtml(discussion.Content)}</p>
        </div>
    `;

    const existingEmpty = discussionsList.querySelector(".empty");
    if (existingEmpty) {
        discussionsList.innerHTML = "";
    }

    discussionsList.prepend(post);
}

if (!!window.EventSource) {
    const source = new EventSource("/events");

    source.addEventListener("discussion", (event) => {
        try {
            const discussion = JSON.parse(event.data);
            addDiscussionToDOM(discussion);
        } catch (err) {
            console.error("Failed to parse live discussion event:", err);
        }
    });

    source.onerror = () => {
        console.warn("Live update connection lost.");
        source.close();
    };
}
