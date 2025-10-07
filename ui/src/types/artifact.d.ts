type ArtifactType = "sdk" | "os-image" | "driver";

export interface Artifact {
  uuid?: string;
  files: string[];
  type: ArtifactType;
  description: string;
  os: OSType;
  architectures: string[];
  versions: string[];
  source_type: SourceType;
  last_updated?: string;
}

export interface ArtifactFormValues {
  type: ArtifactType;
  description: string;
  os: OSType;
  architectures: string;
  versions: string;
  source_type: SourceType;
}
