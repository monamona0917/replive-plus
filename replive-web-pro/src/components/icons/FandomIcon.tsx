import type { SVGProps } from "react";

export const FandomIcon = ({
  className = "w-4 h-4",
  ...props
}: SVGProps<SVGSVGElement>) => {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 100 114"
      className={className}
      fill="none"
      {...props}
    >
      <defs>
        {/* Background Gradient: Electric Blue-Violet to Rich Orchid Purple (Top-Right) to Vivid Magenta-Pink (Bottom) */}
        <linearGradient id="fandomBgGradient" x1="0%" y1="0%" x2="90%" y2="100%">
          <stop offset="0%" stopColor="#3c00eb" />
          <stop offset="22%" stopColor="#6702d0" />
          <stop offset="42%" stopColor="#8c02b2" />
          <stop offset="68%" stopColor="#b40096" />
          <stop offset="88%" stopColor="#d40080" />
          <stop offset="100%" stopColor="#e60074" />
        </linearGradient>

        <filter id="fandomShadow" x="-10%" y="-10%" width="120%" height="125%">
          <feDropShadow
            dx="0"
            dy="2"
            stdDeviation="2"
            floodColor="#000000"
            floodOpacity="0.25"
          />
        </filter>
      </defs>

      {/* Main Shield Body with Outer White Border */}
      <g filter="url(#fandomShadow)">
        <path
          d="M 12 8
             L 88 8
             Q 92 8 92 12
             L 92 86
             L 50 106
             L 8 86
             L 8 12
             Q 8 8 12 8 Z"
          fill="url(#fandomBgGradient)"
          stroke="#ffffff"
          strokeWidth="4.5"
          strokeLinejoin="miter"
        />

        {/* Stylized Exact F Letter (Sleek Top-Right Taper to Reveal Pink-Purple Background) */}
        <path
          d="M 39 25
             C 45 25 68 25 75 25.5
             C 78.5 25.8 80.5 27.5 79.5 30.5
             C 78.5 33 76 35 72.5 35.5
             L 55 35.5
             L 51 52
             L 67 52
             C 70.5 52 72 54 71 56.5
             L 69.5 60.5
             C 68.5 62.5 66.5 63.5 63 63.5
             L 48 63.5
             L 40 89
             L 28.5 89
             L 41.5 48
             C 42.5 44 42 39 39.5 36
             C 37 33.5 34 33.5 31 36
             C 27.5 39.5 26 45 25.5 53
             L 25 61
             C 24.5 64 22.5 65.5 19.5 64
             C 16.5 62.5 15.5 59.5 16 54
             C 16.5 45 20.5 34.5 26 29
             C 30 25.5 34.5 25 39 25 Z"
          fill="#ffffff"
        />
      </g>
    </svg>
  );
};

export default FandomIcon;
