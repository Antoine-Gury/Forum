const homeButton = document.getElementById("homeButton");
const newDiscussionButton = document.getElementById("NewDiscussionButton");
const profileButton = document.getElementById("profileButton");

const navButtons = [homeButton, newDiscussionButton, profileButton];

function setActive(btn) {
    navButtons.forEach(b => b.classList.remove("active"));
    btn.classList.add("active");
}

homeButton.addEventListener("click", () => {
    setActive(homeButton);
    window.location.href = "/";
});

newDiscussionButton.addEventListener("click", () => {
    setActive(newDiscussionButton);
    window.location.href = "/create";
});

profileButton.addEventListener("click", () => {
    setActive(profileButton);
    window.location.href = "/login";
});