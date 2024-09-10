document.addEventListener('DOMContentLoaded', function () {
  document.body.addEventListener('click', (e) => {
    // Check if the clicked element is a link
    const link = e.target.closest('a');
    if (!link) return;

    e.preventDefault();

    if (!document.startViewTransition) {
      return (window.location = link.href);
    }

    console.log('Starting transition');
    document.startViewTransition(() => {
      console.log('Transitioning to', link.href);
      window.location = link.href;
    });
  });
});
