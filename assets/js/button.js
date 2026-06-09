document.addEventListener("DOMContentLoaded", () => {
    const homeButton = document.getElementById("homeButton");
    const newDiscussionButton = document.getElementById("NewDiscussionButton");
    const profileButton = document.getElementById("profileButton");

    const navButtons = [homeButton, newDiscussionButton, profileButton].filter(Boolean);

    function setActive(btn) {
        navButtons.forEach(b => b.classList.remove("active"));
        if (btn) btn.classList.add("active");
    }

    if (homeButton) {
        homeButton.addEventListener("click", () => {
            setActive(homeButton);
            window.location.href = "/";
        });
    }

    if (newDiscussionButton) {
        newDiscussionButton.addEventListener("click", () => {
            setActive(newDiscussionButton);
            window.location.href = "/create";
        });
    }

    if (profileButton) {
        profileButton.addEventListener("click", () => {
            setActive(profileButton);
            window.location.href = "/profil";
        });
    }

});
