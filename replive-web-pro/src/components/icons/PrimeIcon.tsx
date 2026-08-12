import type { SVGProps } from "react";

export const PrimeIcon = ({
  className = "w-4 h-4",
  ...props
}: SVGProps<SVGSVGElement>) => {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 100 100"
      className={className}
      fill="none"
      {...props}
    >
      <defs>
        <linearGradient
          id="primeGoldGradientComp"
          x1="15%"
          y1="10%"
          x2="85%"
          y2="90%"
        >
          <stop offset="0%" stopColor="#fee074" />
          <stop offset="35%" stopColor="#f59e0b" />
          <stop offset="70%" stopColor="#d97706" />
          <stop offset="100%" stopColor="#b45309" />
        </linearGradient>

        <filter
          id="primeSparkleGlowComp"
          x="-20%"
          y="-20%"
          width="140%"
          height="140%"
        >
          <feDropShadow
            dx="0"
            dy="1.5"
            stdDeviation="2"
            floodColor="#b45309"
            floodOpacity="0.3"
          />
        </filter>
      </defs>

      <g filter="url(#primeSparkleGlowComp)">
        {/* Primary Large Sparkle Star (Top-Left) */}
        <path
          d="M 40 8
             C 40 24 54 38 70 38
             C 54 38 40 52 40 68
             C 40 52 26 38 10 38
             C 26 38 40 24 40 8 Z"
          fill="url(#primeGoldGradientComp)"
        />

        {/* Secondary Small Sparkle Star (Bottom-Right) */}
        <path
          d="M 72 48
             C 72 58 81 68 91 68
             C 81 68 72 78 72 88
             C 72 78 63 68 53 68
             C 63 68 72 58 72 48 Z"
          fill="url(#primeGoldGradientComp)"
        />
      </g>
    </svg>
  );
};

export default PrimeIcon;
