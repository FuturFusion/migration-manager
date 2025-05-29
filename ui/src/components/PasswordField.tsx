import { useState } from "react";
import { Form, InputGroup } from "react-bootstrap";
import { GoEye, GoEyeClosed } from "react-icons/go";

const PasswordField = ({ ...passwordFieldAttrs }) => {
  const [showPassword, setShowPassword] = useState(false);

  return (
    <InputGroup>
      <Form.Control
        type={showPassword ? "text" : "password"}
        {...passwordFieldAttrs}
      />
      <InputGroup.Text
        onClick={() => setShowPassword((prev) => !prev)}
        style={{ cursor: "pointer" }}
      >
        {showPassword ? <GoEyeClosed /> : <GoEye />}
      </InputGroup.Text>
    </InputGroup>
  );
};

export default PasswordField;
