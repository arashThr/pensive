document.addEventListener('DOMContentLoaded', () => {
    const slider = document.getElementById('slider');
    const track = document.getElementById('slider-track');
    const prevBtn = document.getElementById('prev-btn');
    const nextBtn = document.getElementById('next-btn');
    const dots = Array.from(document.querySelectorAll('.slider-dot'));
    const captions = Array.from(document.querySelectorAll('#slide-caption span'));

    if (!slider || !track || !prevBtn || !nextBtn || dots.length === 0) return;

    let index = 0;
    const total = dots.length;

    const render = () => {
        track.style.transform = `translateX(${-index * 100}%)`;
        dots.forEach((d, i) => d.classList.toggle('bg-white/60', i === index));
        dots.forEach((d, i) => d.classList.toggle('bg-white/20', i !== index));
        captions.forEach((c, i) => c.classList.toggle('hidden', i !== index));
    };

    const go = (next) => {
        index = (next + total) % total;
        render();
    };

    prevBtn.addEventListener('click', () => go(index - 1));
    nextBtn.addEventListener('click', () => go(index + 1));
    dots.forEach((d) => d.addEventListener('click', () => go(Number(d.dataset.slide || 0))));

    const onKey = (e) => {
        if (e.key === 'ArrowLeft') go(index - 1);
        if (e.key === 'ArrowRight') go(index + 1);
    };
    document.addEventListener('keydown', onKey);

    let timer = setInterval(() => go(index + 1), 5000);
    slider.addEventListener('mouseenter', () => clearInterval(timer));
    slider.addEventListener('mouseleave', () => (timer = setInterval(() => go(index + 1), 5000)));

    let startX = 0;
    track.addEventListener('touchstart', (e) => (startX = e.touches[0].clientX), { passive: true });
    track.addEventListener('touchend', (e) => {
        const endX = e.changedTouches[0].clientX;
        const dx = startX - endX;
        if (Math.abs(dx) < 50) return;
        go(dx > 0 ? index + 1 : index - 1);
    });

    render();
});