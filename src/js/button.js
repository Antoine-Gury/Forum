document.addEventListener('DOMContentLoaded', function () {
    const homeButton = document.getElementById('homeButton');
    const newDiscussionButton = document.getElementById('NewDiscussionButton');
    const profileButton = document.getElementById('profileButton');

    if (homeButton) {
        homeButton.addEventListener('click', function () {
            window.location.href = '/';
        });
    }

    if (newDiscussionButton) {
        newDiscussionButton.addEventListener('click', function () {
            window.location.href = '/create';
        });
    }

    if (profileButton) {
        profileButton.addEventListener('click', function () {
            window.location.href = '/profil';
        });
    }
});
