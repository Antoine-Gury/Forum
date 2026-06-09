document.querySelectorAll(".post").forEach(post => {
    post.addEventListener("mouseover", () => {
        post.style.transform = "scale(1.02)";
        post.style.transition = "0.2s";
    });

    post.addEventListener("mouseout", () => {
        post.style.transform = "scale(1)";
    });
});