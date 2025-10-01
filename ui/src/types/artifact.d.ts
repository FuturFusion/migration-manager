type ArtifactType = "sdk" | "os-image" | "driver";

export interface ArtifactProperties {
  description: string;
  os: OSType;
  architectures: string[];
  versions: string[];
  source_type: SourceType;
}

export interface Artifact {
  uuid?: string;
  files: string[];
  type: ArtifactType;
  properties: ArtifactProperties;
}

export interface ArtifactFormValues {
  type: ArtifactType;
  description: string;
  os: OSType;
  architectures: string;
  versions: string;
  source_type: SourceType;
}
