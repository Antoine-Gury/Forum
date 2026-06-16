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

function avatarHTML(avatarURL) {
    if (avatarURL) {
        return `<img src="${escapeHtml(avatarURL)}" alt="avatar" class="post-avatar-img">`;
    }
    return `⚔`;
}

function makePostClickable(post) {
    const link = post.querySelector("a[href]");
    if (!link) return;
    post.addEventListener("click", (e) => {
        if (e.target.closest("a")) return;
        window.location.href = link.href;
    });
}

function addDiscussionToDOM(discussion) {
    if (!discussionsList) return;

    const post = document.createElement("div");
    post.className = "post";
    post.dataset.score = discussion.Score || 0;
    post.dataset.created = discussion.CreatedAt || new Date().toISOString();
    post.innerHTML = `
        <div class="post-avatar">${avatarHTML(discussion.AvatarURL)}</div>
        <div class="post-body">
            <h3><a href="/discussion?id=${discussion.ID}">${escapeHtml(discussion.Title)}</a></h3>
            <p class="post-meta">Par <strong>${escapeHtml(discussion.Author || "Invité")}</strong></p>
            <p>${escapeHtml(discussion.Content)}</p>
        </div>
    `;

    makePostClickable(post);

    const existingEmpty = discussionsList.querySelector(".empty");
    if (existingEmpty) discussionsList.innerHTML = "";

    discussionsList.prepend(post);
}

document.querySelectorAll(".post").forEach(makePostClickable);

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