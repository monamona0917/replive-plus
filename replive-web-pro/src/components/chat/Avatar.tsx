import { useEffect, useState } from "react";

interface AvatarProps {
  localUrl?: string;
  remoteUrl?: string;
  label: string;
  className: string;
  fallbackClassName?: string;
  loading?: "eager" | "lazy";
  decoding?: "async" | "auto" | "sync";
}

export const Avatar = ({
  localUrl,
  remoteUrl,
  label,
  className,
  fallbackClassName = "",
  loading,
  decoding,
}: AvatarProps) => {
  const [source, setSource] = useState(localUrl || remoteUrl);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    setSource(localUrl || remoteUrl);
    setFailed(false);
  }, [localUrl, remoteUrl]);

  const handleError = () => {
    if (source === localUrl && remoteUrl) {
      setSource(remoteUrl);
      return;
    }
    setFailed(true);
  };

  if (!source || failed) {
    return (
      <div
        className={`${className} ${fallbackClassName} flex items-center justify-center font-semibold select-none`}
        aria-label={label}
      >
        {label.slice(0, 1) || "?"}
      </div>
    );
  }

  return (
    <img
      src={source}
      alt={label}
      className={className}
      loading={loading}
      decoding={decoding}
      onError={handleError}
    />
  );
};

export default Avatar;
