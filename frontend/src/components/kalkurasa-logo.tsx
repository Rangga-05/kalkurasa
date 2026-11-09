import { useId } from "react";

export function KalkurasaLogoIcon({
  className = "h-10 w-10",
}: {
  className?: string;
}) {
  const uid = useId();
  const bgStampId = `bg-stamp-${uid}`;
  const furGradId = `fur-grad-${uid}`;

  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 128 128"
      fill="none"
      role="img"
      aria-label="Logo Kalkurasa"
      className={className}
    >
      <defs>
        <linearGradient
          id={bgStampId}
          x1="12"
          y1="12"
          x2="116"
          y2="116"
          gradientUnits="userSpaceOnUse"
        >
          <stop offset="0%" stopColor="#FDE047" />
          <stop offset="55%" stopColor="#FB923C" />
          <stop offset="100%" stopColor="#EA580C" />
        </linearGradient>
        <linearGradient
          id={furGradId}
          x1="40"
          y1="50"
          x2="88"
          y2="104"
          gradientUnits="userSpaceOnUse"
        >
          <stop offset="0%" stopColor="#64748B" />
          <stop offset="100%" stopColor="#475569" />
        </linearGradient>
      </defs>

      <circle cx="66" cy="66" r="56" fill="#12100E" />
      <circle
        cx="62"
        cy="62"
        r="56"
        fill={`url(#${bgStampId})`}
        stroke="#12100E"
        strokeWidth="5"
      />
      <circle
        cx="62"
        cy="62"
        r="48"
        fill="#FFFDF9"
        stroke="#12100E"
        strokeWidth="3.5"
      />

      {/* Ears */}
      <circle
        cx="35"
        cy="60"
        r="16"
        fill={`url(#${furGradId})`}
        stroke="#12100E"
        strokeWidth="4.2"
      />
      <circle
        cx="36"
        cy="61"
        r="9.5"
        fill="#FDA4AF"
        stroke="#12100E"
        strokeWidth="2.5"
      />

      <circle
        cx="89"
        cy="60"
        r="16"
        fill={`url(#${furGradId})`}
        stroke="#12100E"
        strokeWidth="4.2"
      />
      <circle
        cx="88"
        cy="61"
        r="9.5"
        fill="#FDA4AF"
        stroke="#12100E"
        strokeWidth="2.5"
      />

      {/* Chef Toque Hat */}
      <path
        d="M42 46C35 44 32 36 36 29C39 23 46 22 50 25C53 18 61 15 68 18C73 20 75 24 76 26C82 23 90 27 91 34C92 40 87 45 82 46H42Z"
        fill="#FFFDF9"
        stroke="#12100E"
        strokeWidth="4.2"
        strokeLinejoin="round"
      />
      <path
        d="M51 33V44"
        stroke="#12100E"
        strokeWidth="2.8"
        strokeLinecap="round"
      />
      <path
        d="M62 30V44"
        stroke="#12100E"
        strokeWidth="2.8"
        strokeLinecap="round"
      />
      <path
        d="M73 33V44"
        stroke="#12100E"
        strokeWidth="2.8"
        strokeLinecap="round"
      />

      <rect
        x="41"
        y="43"
        width="42"
        height="10"
        rx="4"
        fill="#FFFDF9"
        stroke="#12100E"
        strokeWidth="4"
      />
      <line
        x1="44"
        y1="48"
        x2="80"
        y2="48"
        stroke="#EA580C"
        strokeWidth="3"
        strokeLinecap="round"
      />

      {/* Head & Muzzle */}
      <path
        d="M40 66C40 54.5 49.5 49 62 49C74.5 49 84 54.5 84 66C87 70 88 77 83 83C78 89 70 93 62 93C54 93 46 89 41 83C36 77 37 70 40 66Z"
        fill={`url(#${furGradId})`}
        stroke="#12100E"
        strokeWidth="4.2"
        strokeLinejoin="round"
      />
      <path
        d="M47 73C47 66 53.5 62 62 62C70.5 62 77 66 77 73C77 82 70 91 62 91C54 91 47 82 47 73Z"
        fill="#CBD5E1"
      />

      {/* Eyes */}
      <ellipse cx="53" cy="65" rx="4.2" ry="5" fill="#12100E" />
      <circle cx="51.8" cy="63.2" r="1.6" fill="#FFFDF9" />
      <ellipse cx="71" cy="65" rx="4.2" ry="5" fill="#12100E" />
      <circle cx="69.8" cy="63.2" r="1.6" fill="#FFFDF9" />

      {/* Whiskers */}
      <path
        d="M43 75L28 72"
        stroke="#12100E"
        strokeWidth="2.6"
        strokeLinecap="round"
      />
      <path
        d="M43 79L27 80"
        stroke="#12100E"
        strokeWidth="2.6"
        strokeLinecap="round"
      />
      <path
        d="M81 75L96 72"
        stroke="#12100E"
        strokeWidth="2.6"
        strokeLinecap="round"
      />
      <path
        d="M81 79L97 80"
        stroke="#12100E"
        strokeWidth="2.6"
        strokeLinecap="round"
      />

      {/* Pink Snout & Smile */}
      <ellipse
        cx="62"
        cy="75"
        rx="6.5"
        ry="4.8"
        fill="#FB7185"
        stroke="#12100E"
        strokeWidth="3.5"
      />
      <ellipse cx="60.5" cy="73.6" rx="2" ry="1.2" fill="#FFFDF9" />
      <path
        d="M56 83C58 86 66 86 68 83"
        stroke="#12100E"
        strokeWidth="3.2"
        strokeLinecap="round"
      />

      {/* Chef Neckerchief */}
      <path
        d="M48 92L62 104L76 92H48Z"
        fill="#EA580C"
        stroke="#12100E"
        strokeWidth="3.5"
        strokeLinejoin="round"
      />
      <circle
        cx="62"
        cy="99"
        r="3.5"
        fill="#FDE047"
        stroke="#12100E"
        strokeWidth="2.5"
      />
    </svg>
  );
}

